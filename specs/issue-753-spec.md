# Specification: Issue #753

## Classification
feature

## Problem Analysis

The issue requests adding a live chat widget (Intercom/Drift) to cloudroof.eu with trial-context prompts. After analyzing the codebase and deployment structure:

1. **No existing chat integration.** The current `public/index.html` has no chat widget code. No environment variables or configuration files reference Intercom or Drift.

2. **No trial-context detection.** The web UI has no concept of "trial" vs. "paid" users. No user state tracking or session management exists in the current static HTML page.

3. **No dynamic backend.** The current `public/index.html` is a static page. The wgmesh binary (`main.go`) only serves pprof and Prometheus metrics endpoints; it does not serve the `public/` directory via HTTP. No web server code exists for cloudroof.eu.

4. **Cloudroof.eu infrastructure is unclear.** While `deploy/lighthouse/setup.sh` references `edge.cloudroof.eu` as a DNS target, the actual web serving infrastructure for cloudroof.eu is not documented in this repository. The lighthouse service is a control plane for xDS/Envoy, not a static file server.

5. **No configuration management.** There's no mechanism to inject API keys or configuration at runtime without code changes. Hardcoding chat widget credentials in the HTML would be a security violation.

The implementation requires:
- Adding chat widget scripts to `public/index.html` with configuration
- Creating a configuration system (environment variables or config file) for API keys
- Detecting trial status for contextual prompts
- Ensuring the static files are served on cloudroof.eu

## Proposed Approach

Add chat widget integration to `public/index.html` with configurable API keys via environment variables. The solution will:

1. **Add chat widget configuration.** Extend `public/index.html` to include Intercom and Drift widget scripts, but only load them when configured via environment variables.

2. **Create configuration file system.** Add a `public/config.js` file that can be generated at build/deploy time with runtime configuration including chat widget credentials.

3. **Add trial-context detection.** Implement basic trial state detection via URL parameters (`?trial=true`) or localStorage, enabling contextual prompts.

4. **Document deployment requirements.** Add deployment documentation explaining how to serve the updated `public/` directory on cloudroof.eu.

5. **Ensure privacy compliance.** Add a cookie consent mechanism before initializing any tracking/chat widgets.

## Implementation Tasks

### Task 1: Create configurable chat widget system in `public/config.js.template`

- **File:** `public/config.js.template` (create)
- **What:** Create a template configuration file that gets populated during deployment with runtime values.
- **Detail:** Create a template file with the following structure:
  ```javascript
  window.WGMESH_CONFIG = {
      chat: {
          intercom: {
              enabled: {{INTERCOM_ENABLED}},
              appId: "{{INTERCOM_APP_ID}}"
          },
          drift: {
              enabled: {{DRIFT_ENABLED}},
              appId: "{{DRIFT_APP_ID}}"
          }
      },
      trial: {
          enabled: true,
          defaultTrial: false
      }
  };
  ```
  The `{{VARIABLE}}` placeholders will be replaced during deployment.

### Task 2: Add chat widget integration to `public/index.html`

- **File:** `public/index.html` (modify)
- **What:** Add chat widget scripts and initialization logic that loads from configuration.
- **Detail:** Before the closing `</body>` tag, add:
  1. Load `config.js` first: `<script src="/config.js"></script>`
  2. Add conditional chat widget initialization scripts:
     ```html
     <script>
     (function() {
         if (!window.WGMESH_CONFIG) return;
         
         // Detect trial status from URL or localStorage
         const isTrial = new URLSearchParams(window.location.search).has('trial') ||
                        localStorage.getItem('wgmesh_trial') === 'true';
         
         // Intercom
         if (window.WGMESH_CONFIG.chat.intercom.enabled && window.WGMESH_CONFIG.chat.intercom.appId) {
             (function(){var w=window;var ic=w.Intercom;if(typeof ic==="function"){ic('reattach_activator',ic('update',w.intercomSettings));return;}var d=document;var i=function(){i.c(arguments)};i.q=[];i.c=function(args){i.q.push(args)};w.Intercom=i;function l(){var s=d.createElement('script');s.type='text/javascript';s.async=true;s.src='https://widget.intercom.io/widget/' + window.WGMESH_CONFIG.chat.intercom.appId;var x=d.getElementsByTagName('script')[0];x.parentNode.insertBefore(s,x);}if(w.attachEvent){w.attachEvent('onload',l);}else{w.addEventListener('load',l,false);}})();
             
             window.intercomSettings = {
                 app_id: window.WGMESH_CONFIG.chat.intercom.appId,
                 name: isTrial ? 'Trial User' : 'Visitor',
                 trial_user: isTrial,
                 created_at: Math.floor(Date.now() / 1000)
             };
         }
         
         // Drift
         if (window.WGMESH_CONFIG.chat.drift.enabled && window.WGMESH_CONFIG.chat.drift.appId) {
             !function(){var t=window.driftt=window.drift=window.driftt||[];if(!t.init){if(t.invoked)return void(window.console&&console.error&&console.error("Drift snippet included twice."));t.invoked=!0;var e=function(){if("function"==typeof t.init)return t.init.apply(t,arguments)},n=[];e.q=e.c=function(t){return n.push(t)},e.l=void 0,e.c=n,e.init=function(e,n){function o(t,e){if(t){var n=t.split(/[,;]/),i=n.length;for(var o=0;o<i;o++)e(n[o])}}var a=this;o(e,function(t){a[t]=function(){return a._.push([t].concat(Array.prototype.slice.call(arguments,0)))}}),this._=[]},new e}();t.SNIPPET_VERSION="0.3.1",t.load=!0,function(t){if(!document.getElementById("drift-widget-script")){var e=document.createElement("script");e.id="drift-widget-script",e.type="text/javascript",e.async=!0,e.src="https://js.driftt.com/include/"+window.WGMESH_CONFIG.chat.drift.appId+".js";var n=document.getElementsByTagName("script")[0];n.parentNode.insertBefore(e,n)}}()}();
             
             drift.on('ready',function(){drift.api.setUser({isTrial:isTrial})});
         }
     })();
     </script>
     ```

### Task 3: Add deployment script for configuration generation

- **File:** `deploy/web/generate-config.sh` (create)
- **What:** Create a script that generates `public/config.js` from the template using environment variables.
- **Detail:** Create a bash script that:
  1. Takes the template file `public/config.js.template`
  2. Replaces `{{INTERCOM_ENABLED}}`, `{{INTERCOM_APP_ID}}`, `{{DRIFT_ENABLED}}`, `{{DRIFT_APP_ID}}` with environment variable values
  3. Outputs `public/config.js`
  4. Makes `true`/`false` strings boolean in JavaScript
  
  Example usage:
  ```bash
  export INTERCOM_ENABLED=true
  export INTERCOM_APP_ID="abc123xyz"
  export DRIFT_ENABLED=false
  ./deploy/web/generate-config.sh
  ```

### Task 4: Create deployment documentation

- **File:** `docs/chat-widget-deployment.md` (create)
- **What:** Document how to deploy the chat widget integration to cloudroof.eu.
- **Detail:** Create documentation covering:
  1. Prerequisites: Intercom/Drift accounts and API keys
  2. Environment variables to set
  3. How to generate `config.js` using the script
  4. How to serve the `public/` directory on cloudroof.eu (nginx, Caddy, or similar)
  5. Testing checklist (verify widget loads, trial context works)
  6. Security considerations (never commit API keys, use environment variables in production)

### Task 5: Add README section for chat widget

- **File:** `README.md` (modify)
- **What:** Document the chat widget feature for developers.
- **Detail:** Add a section "## Chat Widget Integration" that explains:
  - What chat providers are supported
  - How to configure for local development
  - How to obtain API keys
  - Link to `docs/chat-widget-deployment.md` for deployment instructions

### Task 6: Add trial-context URL handling

- **File:** `public/index.html` (modify)
- **What:** Add JavaScript to detect trial context from URL parameters and persist in localStorage.
- **Detail:** Add after the config.js script load:
  ```html
  <script>
  (function() {
      const params = new URLSearchParams(window.location.search);
      if (params.has('trial')) {
          const trialValue = params.get('trial') === 'true' || params.get('trial') === '1';
          localStorage.setItem('wgmesh_trial', trialValue ? 'true' : 'false');
      }
  })();
  </script>
  ```

## Acceptance Criteria

1. **Chat widget loads successfully.** When `INTERCOM_ENABLED=true` and `INTERCOM_APP_ID` is set, the Intercom widget appears on cloudroof.eu. Same for Drift.

2. **Configuration is externalized.** No API keys are hardcoded in `public/index.html` or committed to the repository. All configuration comes from `public/config.js` generated from environment variables.

3. **Trial context is detected.** When a user visits `https://cloudroof.eu/?trial=true`, the chat widget shows contextual prompts for trial users (verified via Intercom/Drift dashboard user attributes).

4. **No configuration means no widget.** If `config.js` is missing or chat is disabled, the page loads normally without errors or broken widgets.

5. **Deployment documented.** `docs/chat-widget-deployment.md` provides clear instructions for deploying to cloudroof.eu.

6. **Privacy compliance.** Cookie consent is shown before initializing any tracking widgets (can be a basic implementation that can be extended later).

7. **Test locally.** Developers can test the chat widget locally by setting environment variables and running the generation script.

8. **README updated.** The `README.md` includes information about the chat widget feature.

## Out of scope

- **User authentication system.** This spec does not include building a full authentication or login system. Trial detection is URL/localStorage-based only.

- **Backend chat service.** We're using third-party services (Intercom/Drift), not building our own chat backend.

- **Advanced cookie consent UI.** Basic consent mechanism only; full GDPR compliance UI is out of scope.

- **Analytics integration.** Chat widget only; no Google Analytics, Mixpanel, or other tracking tools.

- **Real-time trial status from backend.** Trial context is determined client-side via URL parameters or localStorage, not fetched from a backend API.

- **Automated testing of chat widgets.** Manual testing only; no automated browser tests for third-party widgets.

- **Chat widget customization.** Using default widget appearances; no custom styling or advanced configuration.

- **nginx/Caddy configuration.** This spec documents what needs to be served but does not include the web server configuration files for cloudroof.eu.