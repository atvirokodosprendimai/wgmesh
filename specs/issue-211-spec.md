# Specification: Issue #211

## Classification
fix

## Deliverables
code + tests

## Problem Analysis

The `RotationState` struct in `pkg/crypto/rotation.go` has a custom `MarshalJSON` method (lines 99-109) that attempts to serialize the `GracePeriod` field (a `time.Duration`) as a human-readable string instead of nanoseconds. However, the implementation has two critical issues:

### Issue 1: Potential Duplicate JSON Keys

The `MarshalJSON` method uses the type alias pattern with an embedded anonymous struct:

```go
func (rs *RotationState) MarshalJSON() ([]byte, error) {
    type Alias RotationState
    return json.Marshal(&struct {
        GracePeriod string `json:"grace_period"`
        *Alias
    }{
        GracePeriod: rs.GracePeriod.String(),
        Alias:       (*Alias)(rs),
    })
}
```

The embedded `*Alias` includes all fields from `RotationState`, including the original `GracePeriod time.Duration` field with tag `json:"grace_period"`. The anonymous struct also defines a `GracePeriod string` field with the same JSON tag `json:"grace_period"`.

While Go's JSON encoder currently handles this by allowing the outer field to shadow the embedded field, this behavior is fragile and not guaranteed. The JSON RFC allows duplicate keys, and different JSON parsers may handle them differently (last-wins, first-wins, or error). This creates a maintenance risk and potential interoperability issues.

### Issue 2: Missing UnmarshalJSON

There is no corresponding `UnmarshalJSON` method to deserialize the human-readable duration string back into a `time.Duration`. This creates an asymmetric API:

- **Marshal**: Outputs `"grace_period": "1h30m0s"` (human-readable string)
- **Unmarshal**: Expects `"grace_period": 5400000000000` (nanoseconds as int64)

Attempting to unmarshal JSON produced by `MarshalJSON` fails:

```
json: cannot unmarshal string into Go struct field RotationState.grace_period of type time.Duration
```

This breaks round-trip serialization, making it impossible to:
- Persist `RotationState` to JSON files and reload them
- Send `RotationState` over JSON-based APIs and parse it correctly
- Use JSON for configuration or state management

### Current Test Coverage

The existing `TestRotationState` in `rotation_test.go` (lines 65-86) only tests the in-memory methods (`IsInGracePeriod`, `ShouldComplete`) and does not cover JSON marshaling/unmarshaling at all.

## Proposed Approach

### Fix 1: Prevent Duplicate JSON Keys in MarshalJSON

Modify the `MarshalJSON` implementation to explicitly exclude the original `GracePeriod` field from the embedded alias. Two approaches are viable:

**Option A: Add `json:"-"` tag to suppress embedded field**
```go
func (rs *RotationState) MarshalJSON() ([]byte, error) {
    type Alias RotationState
    return json.Marshal(&struct {
        GracePeriod string `json:"grace_period"`
        *Alias      `json:"-"`  // Suppress all embedded fields
        // Re-export other fields manually
        OldSecret string    `json:"old_secret"`
        NewSecret string    `json:"new_secret"`
        StartedAt time.Time `json:"started_at"`
        Completed bool      `json:"completed"`
    }{
        GracePeriod: rs.GracePeriod.String(),
        OldSecret:   rs.OldSecret,
        NewSecret:   rs.NewSecret,
        StartedAt:   rs.StartedAt,
        Completed:   rs.Completed,
    })
}
```

**Option B: Use explicit struct without embedding (Recommended)**
```go
func (rs *RotationState) MarshalJSON() ([]byte, error) {
    return json.Marshal(&struct {
        OldSecret   string    `json:"old_secret"`
        NewSecret   string    `json:"new_secret"`
        GracePeriod string    `json:"grace_period"`
        StartedAt   time.Time `json:"started_at"`
        Completed   bool      `json:"completed"`
    }{
        OldSecret:   rs.OldSecret,
        NewSecret:   rs.NewSecret,
        GracePeriod: rs.GracePeriod.String(),
        StartedAt:   rs.StartedAt,
        Completed:   rs.Completed,
    })
}
```

**Recommendation**: Option B is clearer, more explicit, and easier to maintain. It removes the fragile shadowing behavior entirely.

### Fix 2: Implement UnmarshalJSON

Add a corresponding `UnmarshalJSON` method that parses the human-readable duration string:

```go
func (rs *RotationState) UnmarshalJSON(data []byte) error {
    type Alias RotationState
    aux := &struct {
        GracePeriod string `json:"grace_period"`
        *Alias
    }{
        Alias: (*Alias)(rs),
    }
    
    if err := json.Unmarshal(data, aux); err != nil {
        return err
    }
    
    // Parse the human-readable duration string
    duration, err := time.ParseDuration(aux.GracePeriod)
    if err != nil {
        return fmt.Errorf("invalid grace_period duration: %w", err)
    }
    
    rs.GracePeriod = duration
    return nil
}
```

This method:
1. Uses an auxiliary struct to capture the string `grace_period` field
2. Unmarshals the JSON into the auxiliary struct
3. Parses the duration string using `time.ParseDuration()`
4. Returns a descriptive error if the duration format is invalid

### Alternative: Support Both Formats in UnmarshalJSON

To maintain backward compatibility (in case any existing JSON uses nanoseconds), the `UnmarshalJSON` could handle both string and numeric formats:

```go
func (rs *RotationState) UnmarshalJSON(data []byte) error {
    type Alias RotationState
    aux := &struct {
        GracePeriod interface{} `json:"grace_period"`
        *Alias
    }{
        Alias: (*Alias)(rs),
    }
    
    if err := json.Unmarshal(data, aux); err != nil {
        return err
    }
    
    switch v := aux.GracePeriod.(type) {
    case string:
        duration, err := time.ParseDuration(v)
        if err != nil {
            return fmt.Errorf("invalid grace_period duration: %w", err)
        }
        rs.GracePeriod = duration
    case float64:
        // JSON numbers are decoded as float64
        rs.GracePeriod = time.Duration(v)
    default:
        return fmt.Errorf("grace_period must be a string or number, got %T", v)
    }
    
    return nil
}
```

**Recommendation**: Use the simpler string-only version unless there's evidence of existing JSON files using nanoseconds.

## Affected Files

### Code Changes
- `pkg/crypto/rotation.go`:
  - Lines 99-109: Rewrite `MarshalJSON` to use explicit struct (no embedding)
  - Add new `UnmarshalJSON` method after `MarshalJSON` (approx. 20 lines)

### Test Changes
- `pkg/crypto/rotation_test.go`:
  - Add new test function `TestRotationStateMarshalJSON` covering:
    - Marshal produces correct JSON with human-readable duration
    - Unmarshal correctly parses the JSON back
    - Round-trip preserves all field values
    - Error handling for invalid duration strings

## Test Strategy

### Unit Tests

Add comprehensive test in `pkg/crypto/rotation_test.go`:

```go
func TestRotationStateMarshalJSON(t *testing.T) {
    tests := []struct {
        name        string
        state       *RotationState
        wantErr     bool
        checkFields bool
    }{
        {
            name: "valid state with 90 minute grace period",
            state: &RotationState{
                OldSecret:   "old-secret",
                NewSecret:   "new-secret",
                GracePeriod: 90 * time.Minute,
                StartedAt:   time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
                Completed:   false,
            },
            checkFields: true,
        },
        {
            name: "completed state with 24 hour grace period",
            state: &RotationState{
                OldSecret:   "old",
                NewSecret:   "new",
                GracePeriod: 24 * time.Hour,
                StartedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
                Completed:   true,
            },
            checkFields: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test Marshal
            data, err := json.Marshal(tt.state)
            if err != nil {
                t.Fatalf("Marshal failed: %v", err)
            }

            // Verify JSON contains human-readable duration string
            jsonStr := string(data)
            if !strings.Contains(jsonStr, `"grace_period":"`) {
                t.Errorf("JSON should contain grace_period as string: %s", jsonStr)
            }

            // Verify no duplicate grace_period keys
            // Count occurrences of "grace_period"
            count := strings.Count(jsonStr, `"grace_period"`)
            if count != 1 {
                t.Errorf("Expected exactly 1 grace_period field, found %d in: %s", count, jsonStr)
            }

            // Test Unmarshal
            var state2 RotationState
            err = json.Unmarshal(data, &state2)
            if (err != nil) != tt.wantErr {
                t.Fatalf("Unmarshal error = %v, wantErr %v", err, tt.wantErr)
            }

            if tt.checkFields {
                // Verify round-trip preserves values
                if state2.OldSecret != tt.state.OldSecret {
                    t.Errorf("OldSecret = %v, want %v", state2.OldSecret, tt.state.OldSecret)
                }
                if state2.NewSecret != tt.state.NewSecret {
                    t.Errorf("NewSecret = %v, want %v", state2.NewSecret, tt.state.NewSecret)
                }
                if state2.GracePeriod != tt.state.GracePeriod {
                    t.Errorf("GracePeriod = %v, want %v", state2.GracePeriod, tt.state.GracePeriod)
                }
                if !state2.StartedAt.Equal(tt.state.StartedAt) {
                    t.Errorf("StartedAt = %v, want %v", state2.StartedAt, tt.state.StartedAt)
                }
                if state2.Completed != tt.state.Completed {
                    t.Errorf("Completed = %v, want %v", state2.Completed, tt.state.Completed)
                }
            }
        })
    }
}

func TestRotationStateUnmarshalJSONErrors(t *testing.T) {
    tests := []struct {
        name    string
        json    string
        wantErr bool
    }{
        {
            name:    "invalid duration format",
            json:    `{"old_secret":"old","new_secret":"new","grace_period":"invalid","started_at":"2024-01-01T12:00:00Z","completed":false}`,
            wantErr: true,
        },
        {
            name:    "missing grace_period",
            json:    `{"old_secret":"old","new_secret":"new","started_at":"2024-01-01T12:00:00Z","completed":false}`,
            wantErr: false, // Zero value is acceptable
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            var state RotationState
            err := json.Unmarshal([]byte(tt.json), &state)
            if (err != nil) != tt.wantErr {
                t.Errorf("Unmarshal error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Manual Testing

1. **Verify JSON output format**:
   ```bash
   go test -v -run TestRotationStateMarshalJSON ./pkg/crypto
   ```

2. **Verify round-trip works**:
   ```bash
   go test -race -run TestRotationState ./pkg/crypto
   ```

3. **Check for duplicate keys** using `jq` or similar tools on actual JSON output

### Integration Testing

If `RotationState` is persisted or transmitted over the network, ensure:
- Existing JSON files can still be read (if backward compatibility is needed)
- New JSON files use the string format
- RPC/API endpoints handle the serialization correctly

## Estimated Complexity

**Medium-Low**

### Reasoning:
- **Localized change**: Only affects `RotationState` in a single file
- **Clear solution**: Standard pattern for custom JSON marshaling in Go
- **Test coverage**: Straightforward unit tests with table-driven approach
- **Low risk**: No changes to crypto algorithms, only serialization format
- **No dependencies**: Uses stdlib only (`encoding/json`, `time`)

### Considerations:
- Need to verify if `RotationState` is currently persisted anywhere (files, database, network)
- If existing JSON uses nanoseconds, may need backward-compatible `UnmarshalJSON`
- Should check if any code depends on the current marshaling format

**Estimated Time:** 1-2 hours including testing

## Additional Notes

### Why Human-Readable Durations?

The intention of the original `MarshalJSON` was to produce human-readable JSON output like:
```json
{
  "grace_period": "24h0m0s"
}
```

Instead of the default:
```json
{
  "grace_period": 86400000000000
}
```

This is more user-friendly for:
- Configuration files
- Log inspection
- Debugging
- Manual JSON editing

### Go Duration String Format

`time.Duration.String()` formats durations as combinations of:
- `72h` (hours)
- `30m` (minutes)  
- `45s` (seconds)
- `500ms` (milliseconds)
- `100µs` (microseconds)
- `250ns` (nanoseconds)

Examples:
- `90 * time.Minute` → `"1h30m0s"`
- `24 * time.Hour` → `"24h0m0s"`
- `45 * time.Second` → `"45s"`

`time.ParseDuration()` accepts the same format for round-trip compatibility.

### Backward Compatibility Considerations

If there are existing JSON files or network messages using `RotationState`:
1. Check if they exist (search codebase for persistence/serialization of `RotationState`)
2. If yes, implement the dual-format `UnmarshalJSON` (handles both string and number)
3. If no, use the simpler string-only version
4. Consider a migration path if needed (read old format, write new format)

### Related Patterns

This is a common pattern in Go when customizing JSON output for `time.Duration`:
- Standard library doesn't provide custom marshalers for `time.Duration`
- Default behavior is nanoseconds as int64
- Custom marshalers enable human-readable strings
- Always implement both `MarshalJSON` and `UnmarshalJSON` for symmetry

### Security Considerations

- `time.ParseDuration` is safe and validates input format
- No injection risks (duration format is strictly defined)
- Errors are properly wrapped and returned
- No impact on cryptographic operations (only affects serialization)
