# Specification: Issue #189

## Classification
feature

## Deliverables
code

## Problem Analysis

### Current State

PR #188 added mem0 persistent memory for Goose CI runs. The system works as follows:

1. **mem0 Integration**: Uses mem0 OSS as a semantic memory layer to help Goose learn from past implementation runs
2. **Storage**: Memories are stored in a Qdrant vector database at `/tmp/mem0-qdrant`
3. **Caching**: The Qdrant directory is cached via GitHub Actions cache using `actions/cache/restore@v4` and `actions/cache/save@v4`
4. **Sanitization**: Sensitive data is sanitized before storage via `sanitize_text()` in `.github/scripts/mem0-save.py` (lines 28-47)
   - Strips OpenAI keys, mem0 keys, GitHub PATs, Bearer tokens, URLs with credentials, env var leaks
5. **Cache Key**: Uses a stable key `mem0-db-${{ github.repository }}-v1` that doesn't change with dependencies

### Security Gap

While `sanitize_text()` prevents secrets from being stored in memory entries, the **cache artifact itself is not encrypted at rest**. This creates a defense-in-depth gap:

- If the GitHub Actions cache is compromised or exfiltrated, the vector database could be readable
- Even though sanitization prevents direct secret leakage, memory entries might contain:
  - Code patterns and implementation details from private repositories
  - Project structure and architectural decisions
  - Error messages that could reveal internal infrastructure
  - Developer names and workflow patterns
- For repositories handling sensitive codebases (finance, healthcare, government), this represents an unnecessary risk

### Security Model

Current protection:
- **Layer 1**: GitHub Actions cache access control (restricts who can download cache)
- **Layer 2**: `sanitize_text()` removes obvious secrets before storage

Proposed addition:
- **Layer 3**: Encryption at rest for the cache artifact itself

This follows the **defense-in-depth** principle: even if cache access controls fail AND a secret bypasses sanitization, the encrypted artifact remains protected.

## Proposed Approach

Add AES-256-CBC encryption to the Qdrant directory before caching and decrypt after restore.

### Implementation Steps

#### 1. Add GitHub Secret

Repository administrators must add a new GitHub Actions secret:
- **Name**: `MEM0_ENCRYPTION_KEY`
- **Purpose**: AES-256 encryption key for mem0 cache
- **Format**: Any string (OpenSSL's PBKDF2 will derive the actual key from this passphrase)
- **Recommendation**: Generate using `openssl rand -base64 32` for strong entropy

#### 2. Modify goose-build.yml Workflow

**After mem0-save step** (currently around line 685):
```yaml
- name: Encrypt mem0 database for caching
  if: always()
  env:
    MEM0_ENCRYPTION_KEY: ${{ secrets.MEM0_ENCRYPTION_KEY }}
  run: |
    # Only encrypt if the database directory exists and key is set
    if [ -d "/tmp/mem0-qdrant" ] && [ -n "$MEM0_ENCRYPTION_KEY" ]; then
      echo "Encrypting mem0 database..."
      # Create tarball and encrypt in one pass
      tar czf - /tmp/mem0-qdrant | \
        openssl enc -aes-256-cbc -salt -pbkdf2 -iter 100000 \
        -pass env:MEM0_ENCRYPTION_KEY \
        -out /tmp/mem0-encrypted.tar.gz.enc
      
      # Verify encrypted file was created
      if [ -f "/tmp/mem0-encrypted.tar.gz.enc" ]; then
        echo "Encrypted: $(du -h /tmp/mem0-encrypted.tar.gz.enc | cut -f1)"
        # Remove plaintext to ensure only encrypted version is cached
        rm -rf /tmp/mem0-qdrant
      else
        echo "::warning::Encryption failed, will cache plaintext as fallback"
      fi
    else
      echo "::notice::Skipping encryption (database not found or key not set)"
    fi
```

**Modify cache save step** (currently around line 688):
```yaml
- name: Save mem0 memory cache
  if: always()
  uses: actions/cache/save@v4
  with:
    # Cache encrypted file if it exists, otherwise fall back to plaintext dir
    path: |
      /tmp/mem0-encrypted.tar.gz.enc
      /tmp/mem0-qdrant
    key: mem0-db-${{ github.repository }}-v1
```

**Before mem0-retrieve step** (currently around line 140):
```yaml
- name: Decrypt mem0 database after restore
  env:
    MEM0_ENCRYPTION_KEY: ${{ secrets.MEM0_ENCRYPTION_KEY }}
  run: |
    # Only decrypt if encrypted file exists and key is set
    if [ -f "/tmp/mem0-encrypted.tar.gz.enc" ] && [ -n "$MEM0_ENCRYPTION_KEY" ]; then
      echo "Decrypting mem0 database..."
      # Decrypt and extract in one pass
      openssl enc -d -aes-256-cbc -pbkdf2 -iter 100000 \
        -pass env:MEM0_ENCRYPTION_KEY \
        -in /tmp/mem0-encrypted.tar.gz.enc | \
        tar xzf - -C /
      
      if [ -d "/tmp/mem0-qdrant" ]; then
        echo "Decrypted: $(du -sh /tmp/mem0-qdrant | cut -f1)"
      else
        echo "::warning::Decryption failed or produced no output"
      fi
    elif [ -f "/tmp/mem0-encrypted.tar.gz.enc" ]; then
      echo "::warning::Encrypted cache found but MEM0_ENCRYPTION_KEY not set, cannot decrypt"
    else
      echo "::notice::No encrypted cache found (using plaintext or first run)"
    fi
```

### Migration Strategy

The implementation is **backward compatible** and supports gradual rollout:

1. **Before adding secret**: Workflow continues to cache plaintext directory (current behavior)
2. **After adding secret**: Workflow automatically switches to encrypted caching
3. **Migration**: 
   - First run after adding secret will cache both plaintext (from restore) and encrypted
   - Second run will restore encrypted version and cache only encrypted
   - Plaintext will naturally age out of cache

### Encryption Parameters

- **Algorithm**: AES-256-CBC (widely supported, battle-tested)
- **Key Derivation**: PBKDF2 with 100,000 iterations (OpenSSL default for `-pbkdf2`)
- **Salt**: OpenSSL's `-salt` flag automatically adds random salt to prevent rainbow table attacks
- **Compression**: gzip compression before encryption reduces cache size and transfer time

### Error Handling

All encryption/decryption steps are **non-fatal**:
- If encryption fails, workflow falls back to caching plaintext directory
- If decryption fails, workflow continues (first run scenario or corrupted cache)
- Warnings are logged via `::warning::` for visibility in GitHub Actions logs
- Notices are logged via `::notice::` for informational messages

## Affected Files

### Code Changes

1. **`.github/workflows/goose-build.yml`** (3 modifications):
   - Add "Encrypt mem0 database for caching" step after mem0-save (around line 685)
   - Modify "Save mem0 memory cache" step to cache encrypted file (around line 688)
   - Add "Decrypt mem0 database after restore" step before mem0-retrieve (around line 140)

### Documentation Changes

**None required** - This is an internal implementation detail. If administrators ask about the `MEM0_ENCRYPTION_KEY` secret, it's self-documenting via workflow YAML comments.

Optional: Could add a note to `.github/workflows/goose-build.yml` header comments documenting the secret.

### Configuration Changes

**Repository secrets** (manual setup by admin):
- Add `MEM0_ENCRYPTION_KEY` in GitHub repository settings → Secrets and variables → Actions

## Test Strategy

### Pre-Implementation Testing (Verify Current State)

1. **Verify current cache behavior**:
   - Trigger goose-build workflow manually
   - Check Actions cache for `mem0-db-atvirokodosprendimai/wgmesh-v1`
   - Download cache artifact (if accessible) and verify it contains plaintext Qdrant directory

### Post-Implementation Testing

#### Phase 1: Secret Not Set (Backward Compatibility)

1. **Run workflow WITHOUT adding `MEM0_ENCRYPTION_KEY` secret**
2. **Expected behavior**:
   - Encryption step logs: `::notice::Skipping encryption (database not found or key not set)`
   - Cache saves plaintext `/tmp/mem0-qdrant` directory (current behavior)
   - Decryption step logs: `::notice::No encrypted cache found (using plaintext or first run)`
   - Workflow succeeds normally

#### Phase 2: First Run with Encryption

1. **Add `MEM0_ENCRYPTION_KEY` secret** to repository
2. **Run workflow (first time with secret)**
3. **Expected behavior**:
   - Encryption step creates `/tmp/mem0-encrypted.tar.gz.enc`
   - Cache saves both encrypted file AND plaintext directory (migration)
   - Logs show encrypted file size
   - Workflow succeeds

#### Phase 3: Subsequent Runs (Steady State)

1. **Run workflow again**
2. **Expected behavior**:
   - Cache restore downloads `/tmp/mem0-encrypted.tar.gz.enc`
   - Decryption step extracts to `/tmp/mem0-qdrant`
   - mem0-retrieve reads from decrypted directory successfully
   - After mem0-save, re-encrypts and caches only encrypted file
   - Workflow succeeds

#### Phase 4: Decryption Failure (Wrong Key)

1. **Change `MEM0_ENCRYPTION_KEY` to a different value**
2. **Run workflow**
3. **Expected behavior**:
   - Decryption fails with OpenSSL error
   - Warning logged: `::warning::Decryption failed or produced no output`
   - mem0-retrieve fails gracefully (logs "mem0 database not found (first run?)")
   - Workflow continues (non-fatal)
   - New encrypted cache created with new key

### Manual Verification

After implementation, verify encrypted cache artifact:

1. **Download cache artifact** (if accessible):
   ```bash
   # This requires admin access to Actions cache
   gh cache list
   gh cache download mem0-db-atvirokodosprendimai/wgmesh-v1
   ```

2. **Verify it's encrypted**:
   ```bash
   file mem0-encrypted.tar.gz.enc
   # Should output: "data" or "openssl encrypted data"
   # Should NOT output: "gzip compressed" or "POSIX tar archive"
   ```

3. **Verify decryption works locally**:
   ```bash
   # Using the same key
   export MEM0_ENCRYPTION_KEY="your-secret-key"
   openssl enc -d -aes-256-cbc -pbkdf2 -iter 100000 \
     -pass env:MEM0_ENCRYPTION_KEY \
     -in mem0-encrypted.tar.gz.enc | tar tzf -
   # Should list files from /tmp/mem0-qdrant
   ```

### Performance Testing

Measure CI impact by comparing workflow run times:

1. **Baseline**: Average runtime of 5 recent workflow runs (before encryption)
2. **After encryption**: Average runtime of 5 runs with encryption enabled
3. **Expected delta**: +20-40 seconds (10-20s encrypt, 10-20s decrypt)
   - Actual time depends on Qdrant DB size
   - gzip compression may reduce cache transfer time, partially offsetting encryption overhead

## Estimated Complexity

**low** (1-2 hours)

### Rationale

- **Simple change**: Only modifies GitHub Actions workflow YAML
- **No code changes**: No Go code modifications required
- **Standard tools**: Uses OpenSSL (pre-installed in GitHub Actions runners)
- **Non-breaking**: Backward compatible with graceful degradation
- **Low risk**: All steps are non-fatal; failures fall through to existing behavior
- **Well-understood**: AES-256-CBC with PBKDF2 is a standard, battle-tested approach

### Time Breakdown

- Understanding current workflow and mem0 integration: 20 minutes (already done in spec)
- Writing encryption/decryption steps in YAML: 15 minutes
- Testing backward compatibility (no secret): 10 minutes
- Testing encryption (first run): 10 minutes
- Testing decryption (subsequent runs): 10 minutes
- Testing failure cases and warnings: 10 minutes
- Documentation review and PR submission: 5 minutes
- **Total**: ~80 minutes

### Trade-offs Acknowledged

**Pros**:
- Defense-in-depth: Protects against cache exfiltration
- Compliance-ready: Meets encryption-at-rest requirements for sensitive codebases
- Zero trust: Doesn't rely solely on GitHub's access controls
- Future-proof: Prepares for handling more sensitive repositories

**Cons**:
- **CI time increase**: ~30 seconds per run (20s measured in issue, could be up to 40s with larger DBs)
  - Mitigation: This is acceptable for security hardening; time can be reduced with parallel steps if needed
- **Secret management**: Adds another secret to manage
  - Mitigation: Standard practice; uses GitHub Secrets (encrypted, audit-logged)
- **No incremental updates**: Must encrypt/decrypt full DB each time
  - Mitigation: Qdrant DB is relatively small (~10-50MB); full encryption is acceptable
  - Alternative: Could explore streaming encryption for very large DBs in the future
- **Key rotation complexity**: Changing the key invalidates cached DB
  - Mitigation: This is actually a feature (forces fresh start); document in workflow comments

### Risk Assessment

**Low risk**:
- Changes are isolated to CI workflow (no production code impact)
- Non-fatal error handling prevents workflow breakage
- Backward compatible (works with or without secret)
- Standard encryption (OpenSSL AES-256-CBC is well-tested)
- Rollback is trivial (remove the secret or revert YAML changes)

## Priority

**Low** (as stated in issue) - Sanitization already prevents secret leakage. This is hardening for defense-in-depth and preparing for more sensitive codebases.

## Dependencies

- ✅ PR #188 (mem0 integration) - **MERGED** (commit f97602c)
- GitHub Actions runner must have `openssl` command (pre-installed in `ubuntu-latest`)
- Repository admin must add `MEM0_ENCRYPTION_KEY` secret (can be done anytime; workflow degrades gracefully without it)
