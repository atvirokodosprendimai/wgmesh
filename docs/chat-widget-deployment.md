# Chat Widget Deployment Guide

This guide explains how to deploy and configure the chat widget integration on cloudroof.eu.

## Overview

The wgmesh website supports two chat widget providers: Intercom and Drift. Both can be enabled simultaneously or individually. Chat widgets are configured via runtime configuration and can display contextual prompts based on trial status.

## Prerequisites

### Intercom Setup

1. Create an account at [intercom.com](https://www.intercom.com/)
2. Create a new app or use an existing one
3. Navigate to Settings > Installation > Web
4. Copy your **App ID** (e.g., `abc123xyz`)

### Drift Setup

1. Create an account at [drift.com](https://www.drift.com/)
2. Navigate to Settings > Widget > Install
3. Copy your **Widget ID** (e.g., `def456uvw`)

## Configuration

### Environment Variables

Set the following environment variables before generating the configuration:

```bash
# Enable Intercom and set its App ID
export INTERCOM_ENABLED=true
export INTERCOM_APP_ID="your-intercom-app-id"

# Enable Drift and set its Widget ID
export DRIFT_ENABLED=true
export DRIFT_APP_ID="your-drift-widget-id"
```

### Generating config.js

Run the configuration generation script from the project root:

```bash
./deploy/web/generate-config.sh
```

This script:
1. Reads `public/config.js.template`
2. Replaces placeholders with environment variable values
3. Writes the result to `public/config.js`

The generated `public/config.js` file should **not** be committed to the repository, as it contains API keys.

### Disabling Chat Widgets

To disable chat widgets, set the enabled flags to `false` or omit them:

```bash
export INTERCOM_ENABLED=false
export DRIFT_ENABLED=false
./deploy/web/generate-config.sh
```

The page will load normally without initializing any chat widgets.

## Trial Context

Chat widgets can display contextual prompts based on whether a user is a trial user.

### How Trial Detection Works

Trial status is determined client-side:

1. **URL Parameter**: Add `?trial=true` to any page URL
   - Example: `https://cloudroof.eu/?trial=true`
2. **LocalStorage**: The trial flag is persisted in localStorage for future visits
3. **Chat Widget User Attributes**: When a chat widget initializes, it sets:
   - Intercom: `trial_user: true/false`
   - Drift: `isTrial: true/false`

### Using Trial Context

Link to your site with the trial parameter:

```html
<a href="https://cloudroof.eu/?trial=true">Start Free Trial</a>
```

The chat widget will then show trial-specific messaging configured in your Intercom/Drift dashboard.

## Deployment to cloudroof.eu

### Option 1: nginx

Configure nginx to serve the `public/` directory:

```nginx
server {
    listen 80;
    listen [::]:80;
    server_name cloudroof.eu;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name cloudroof.eu;

    ssl_certificate /etc/ssl/certs/cloudroof.eu.crt;
    ssl_certificate_key /etc/ssl/private/cloudroof.eu.key;

    root /var/www/cloudroof.eu;
    index index.html;

    location / {
        try_files $uri $uri/ =404;
    }

    # Cache config.js but allow revalidation
    location = /config.js {
        expires 1h;
        add_header Cache-Control "public, must-revalidate";
    }
}
```

Deploy the files:

```bash
# Generate config.js with production credentials
export INTERCOM_ENABLED=true
export INTERCOM_APP_ID="production-app-id"
export DRIFT_ENABLED=false
./deploy/web/generate-config.sh

# Copy to server
rsync -avz public/ user@cloudroof.eu:/var/www/cloudroof.eu/
```

### Option 2: Caddy

Caddy automatically provisions HTTPS certificates:

```caddyfile
cloudroof.eu {
    root * /var/www/cloudroof.eu
    file_server
}
```

### Option 3: Static Hosting (Netlify, Vercel, Cloudflare Pages)

1. Build your site with config.js generated
2. Deploy the `public/` directory
3. Environment variables can be set in the hosting platform's build settings

### CI/CD Integration

Add config generation to your deployment pipeline:

```yaml
# GitHub Actions example
- name: Generate chat config
  env:
    INTERCOM_ENABLED: true
    INTERCOM_APP_ID: ${{ secrets.INTERCOM_APP_ID }}
    DRIFT_ENABLED: false
  run: |
    ./deploy/web/generate-config.sh

- name: Deploy to cloudroof.eu
  run: |
    rsync -avz public/ ${{ secrets.DEPLOY_USER }}@cloudroof.eu:/var/www/cloudroof.eu/
```

## Testing Checklist

### Local Testing

1. **Test with widgets enabled:**
   ```bash
   export INTERCOM_ENABLED=true
   export INTERCOM_APP_ID="test-id"
   ./deploy/web/generate-config.sh
   python3 -m http.server 8000 --directory public
   # Visit http://localhost:8000
   # Verify: Chat widget appears
   ```

2. **Test with widgets disabled:**
   ```bash
   export INTERCOM_ENABLED=false
   ./deploy/web/generate-config.sh
   # Verify: Page loads normally, no console errors
   ```

3. **Test trial context:**
   - Visit `http://localhost:8000/?trial=true`
   - Open browser console and check: `localStorage.getItem('wgmesh_trial') === 'true'`
   - Reload page without parameter — trial status persists

4. **Test missing config.js:**
   ```bash
   rm public/config.js
   # Visit http://localhost:8000
   # Verify: Page loads normally, no console errors
   ```

### Production Testing

1. **Check config.js is served:**
   ```bash
   curl -I https://cloudroof.eu/config.js
   # Should return 200 OK
   ```

2. **Verify widget loads:**
   - Open browser DevTools > Network
   - Visit https://cloudroof.eu
   - Check for requests to `widget.intercom.io` or `js.driftt.com`

3. **Verify trial context:**
   - Visit https://cloudroof.eu/?trial=true
   - Open Intercom/Drift dashboard
   - Check user attributes for `trial_user: true` or `isTrial: true`

## Security Considerations

### Never Commit API Keys

- `public/config.js` must be in `.gitignore`
- Only commit `public/config.js.template`
- Use environment variables in production

### .gitignore Configuration

Ensure your `.gitignore` contains:

```
public/config.js
```

### Environment Variable Management

- Use secret management for production deployments (e.g., GitHub Secrets, AWS Secrets Manager, Vault)
- Rotate API keys regularly
- Use different app IDs for development and production

### HTTPS Required

- Chat widgets require HTTPS in production
- Both Intercom and Drift may refuse to load on HTTP pages
- Use nginx or Caddy for automatic HTTPS

## Troubleshooting

### Widget Not Appearing

1. Check browser console for errors
2. Verify `config.js` is being served (check Network tab)
3. Verify environment variables were set correctly
4. Check that API keys are valid in Intercom/Drift dashboard

### Console Errors

**"WGMESH_CONFIG is not defined":**
- Ensure `config.js` loads before the trial detection script
- Check that `/config.js` returns 200 OK

**"Intercom/Drift snippet included twice":**
- Ensure you haven't added other chat widget scripts
- Check for conflicts with other plugins

### Trial Context Not Working

1. Open browser console and run: `localStorage.getItem('wgmesh_trial')`
2. If `null`, visit `?trial=true` to set it
3. Check that the URL parameter is `?trial=true` (not `?trial=1` sets `true`, both work)

## Cookie Consent (Future Enhancement)

The current implementation initializes chat widgets immediately. For full GDPR compliance, implement a cookie consent banner before loading widget scripts. This is planned for a future update.

## Support

For issues or questions:
- Open an issue on [GitHub](https://github.com/atvirokodosprendimai/wgmesh/issues)
- Intercom: Use the widget on cloudroof.eu
- Drift: Use the widget on cloudroof.eu
