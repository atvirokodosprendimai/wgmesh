#!/usr/bin/env python3
"""Generate an HTML test report from NDJSON trace events.

Usage:
    python3 gen-report.py <trace.jsonl> <output.html>

Reads trace events emitted by lib.sh's emit_event() function and generates
a self-contained HTML report with:
  - Test execution timeline (Gantt-style bars)
  - Chaos event overlay
  - Data plane metrics (transfer, iperf, MTU results)
  - Duration breakdown per test
  - Tier summary statistics
"""

import json
import sys
from datetime import datetime, timezone
from pathlib import Path


def load_events(path: str) -> list[dict]:
    """Load NDJSON events from trace file."""
    events = []
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                events.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    return events


def build_test_timeline(events: list[dict]) -> list[dict]:
    """Pair test_start/test_end events into timeline entries."""
    starts = {}
    timeline = []
    for ev in events:
        if ev["type"] == "test_start":
            starts[ev["name"]] = ev
        elif ev["type"] == "test_end":
            start_ev = starts.pop(ev["name"], None)
            start_ts = start_ev["ts"] if start_ev else ev["ts"]
            timeline.append({
                "id": ev["name"],
                "test_name": ev.get("name", ""),
                "tier": ev.get("tier", "?"),
                "start": start_ts,
                "end": ev["ts"],
                "duration": float(ev.get("duration", ev["ts"] - start_ts)),
                "result": ev.get("result", "?"),
            })
    return timeline


def build_chaos_events(events: list[dict]) -> list[dict]:
    """Extract chaos apply/clear events."""
    chaos = []
    for ev in events:
        if ev["type"] in ("chaos_apply", "chaos_clear"):
            chaos.append({
                "ts": ev["ts"],
                "type": ev["type"],
                "node": ev["name"],
                "chaos_type": ev.get("type_param", ev.get("type", "")),
                "params": ev.get("params", ""),
                "tier": ev.get("tier", "?"),
            })
    return chaos


def build_data_plane_metrics(events: list[dict]) -> list[dict]:
    """Extract data plane verification results."""
    metrics = []
    for ev in events:
        if ev["type"].startswith("data_"):
            metrics.append({
                "ts": ev["ts"],
                "type": ev["type"],
                "name": ev["name"],
                "tier": ev.get("tier", "?"),
                **{k: v for k, v in ev.items()
                   if k not in ("ts", "type", "name", "tier")},
            })
    return metrics


def build_tier_summary(events: list[dict]) -> list[dict]:
    """Extract tier start/end timing."""
    starts = {}
    tiers = []
    for ev in events:
        if ev["type"] == "tier_start":
            starts[ev["name"]] = ev
        elif ev["type"] == "tier_end":
            start_ev = starts.pop(ev["name"], None)
            start_ts = start_ev["ts"] if start_ev else ev["ts"]
            tiers.append({
                "name": ev["name"],
                "start": start_ts,
                "end": ev["ts"],
                "duration": ev["ts"] - start_ts,
                "tests": start_ev.get("tests", "?") if start_ev else "?",
            })
    return tiers


def format_duration(seconds: float) -> str:
    """Format seconds as human-readable string."""
    if seconds < 60:
        return f"{seconds:.0f}s"
    m, s = divmod(int(seconds), 60)
    if m < 60:
        return f"{m}m{s:02d}s"
    h, m = divmod(m, 60)
    return f"{h}h{m:02d}m{s:02d}s"


def generate_html(events: list[dict]) -> str:
    """Generate self-contained HTML report."""
    timeline = build_test_timeline(events)
    chaos = build_chaos_events(events)
    data_metrics = build_data_plane_metrics(events)
    tiers = build_tier_summary(events)

    if not timeline:
        return "<html><body><h1>No test events found</h1></body></html>"

    # Calculate global time range
    all_times = [t["start"] for t in timeline] + [t["end"] for t in timeline]
    t_min = min(all_times)
    t_max = max(all_times)
    t_range = max(t_max - t_min, 1)

    # Color map
    colors = {"PASS": "#22c55e", "FAIL": "#ef4444", "SKIP": "#eab308"}

    # Build Gantt bars
    gantt_bars = []
    for t in timeline:
        left_pct = ((t["start"] - t_min) / t_range) * 100
        width_pct = max(((t["end"] - t["start"]) / t_range) * 100, 0.5)
        color = colors.get(t["result"], "#6b7280")
        gantt_bars.append(f"""
        <div class="gantt-row">
            <div class="gantt-label">{t['id']}</div>
            <div class="gantt-track">
                <div class="gantt-bar" style="left:{left_pct:.2f}%;width:{width_pct:.2f}%;background:{color}"
                     title="{t['id']}: {format_duration(t['duration'])} ({t['result']})">
                    {format_duration(t['duration'])}
                </div>
            </div>
        </div>""")

    # Build test results table
    test_rows = []
    for t in timeline:
        badge = f'<span class="badge" style="background:{colors.get(t["result"], "#6b7280")}">{t["result"]}</span>'
        test_rows.append(f"""
        <tr>
            <td>{t['id']}</td>
            <td>Tier {t['tier']}</td>
            <td>{badge}</td>
            <td>{format_duration(t['duration'])}</td>
        </tr>""")

    # Build data plane table
    dp_rows = []
    for m in data_metrics:
        dp_rows.append(f"""
        <tr>
            <td>{m['type']}</td>
            <td>{m['name']}</td>
            <td>Tier {m['tier']}</td>
            <td>{m.get('result', m.get('mbps', '-'))}</td>
            <td>{m.get('size_mb', m.get('payload', m.get('duration', '-')))}</td>
        </tr>""")

    # Build tier summary
    tier_rows = []
    for t in tiers:
        tier_rows.append(f"""
        <tr>
            <td>{t['name']}</td>
            <td>{t['tests']}</td>
            <td>{format_duration(t['duration'])}</td>
        </tr>""")

    # Chaos events log
    chaos_rows = []
    for c in chaos:
        rel_time = format_duration(c["ts"] - t_min)
        chaos_rows.append(f"""
        <tr>
            <td>{rel_time}</td>
            <td>Tier {c['tier']}</td>
            <td>{c['type']}</td>
            <td>{c['node']}</td>
            <td>{c.get('params', '')}</td>
        </tr>""")

    # Stats
    total = len(timeline)
    passed = sum(1 for t in timeline if t["result"] == "PASS")
    failed = sum(1 for t in timeline if t["result"] == "FAIL")
    skipped = sum(1 for t in timeline if t["result"] == "SKIP")
    total_duration = format_duration(t_range)

    generated = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")

    return f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>wgmesh Integration Test Report</title>
<style>
    * {{ margin: 0; padding: 0; box-sizing: border-box; }}
    body {{ font-family: -apple-system, system-ui, sans-serif; background: #0f172a; color: #e2e8f0; padding: 2rem; }}
    h1 {{ color: #f1f5f9; margin-bottom: 0.5rem; }}
    h2 {{ color: #94a3b8; margin: 2rem 0 1rem; border-bottom: 1px solid #334155; padding-bottom: 0.5rem; }}
    .stats {{ display: flex; gap: 1.5rem; margin: 1rem 0 2rem; flex-wrap: wrap; }}
    .stat {{ background: #1e293b; padding: 1rem 1.5rem; border-radius: 8px; min-width: 120px; }}
    .stat-value {{ font-size: 2rem; font-weight: 700; }}
    .stat-label {{ color: #94a3b8; font-size: 0.85rem; }}
    .pass {{ color: #22c55e; }}
    .fail {{ color: #ef4444; }}
    .skip {{ color: #eab308; }}
    table {{ width: 100%; border-collapse: collapse; background: #1e293b; border-radius: 8px; overflow: hidden; margin-bottom: 1rem; }}
    th {{ background: #334155; padding: 0.75rem 1rem; text-align: left; font-size: 0.85rem; color: #94a3b8; }}
    td {{ padding: 0.5rem 1rem; border-top: 1px solid #334155; font-size: 0.9rem; }}
    tr:hover {{ background: #334155; }}
    .badge {{ padding: 2px 8px; border-radius: 4px; color: white; font-size: 0.8rem; font-weight: 600; }}
    .gantt-row {{ display: flex; align-items: center; margin: 2px 0; }}
    .gantt-label {{ width: 80px; font-size: 0.8rem; color: #94a3b8; text-align: right; padding-right: 8px; flex-shrink: 0; }}
    .gantt-track {{ flex: 1; height: 24px; background: #1e293b; border-radius: 4px; position: relative; overflow: hidden; }}
    .gantt-bar {{ position: absolute; height: 100%; border-radius: 4px; display: flex; align-items: center; padding: 0 6px;
                  font-size: 0.7rem; color: white; white-space: nowrap; overflow: hidden; min-width: 4px; }}
    .footer {{ margin-top: 3rem; color: #475569; font-size: 0.8rem; text-align: center; }}
    .section {{ margin-bottom: 2rem; }}
    .empty {{ color: #64748b; font-style: italic; padding: 1rem; }}
</style>
</head>
<body>
<h1>wgmesh Integration Test Report</h1>
<p style="color:#64748b">Generated: {generated}</p>

<div class="stats">
    <div class="stat"><div class="stat-value">{total}</div><div class="stat-label">Total Tests</div></div>
    <div class="stat"><div class="stat-value pass">{passed}</div><div class="stat-label">Passed</div></div>
    <div class="stat"><div class="stat-value fail">{failed}</div><div class="stat-label">Failed</div></div>
    <div class="stat"><div class="stat-value skip">{skipped}</div><div class="stat-label">Skipped</div></div>
    <div class="stat"><div class="stat-value">{total_duration}</div><div class="stat-label">Duration</div></div>
</div>

<div class="section">
<h2>Test Timeline</h2>
{''.join(gantt_bars) if gantt_bars else '<p class="empty">No timing data</p>'}
</div>

<div class="section">
<h2>Tier Summary</h2>
{'<table><tr><th>Tier</th><th>Tests</th><th>Duration</th></tr>' + ''.join(tier_rows) + '</table>' if tier_rows else '<p class="empty">No tier data</p>'}
</div>

<div class="section">
<h2>Test Results</h2>
<table>
<tr><th>ID</th><th>Tier</th><th>Result</th><th>Duration</th></tr>
{''.join(test_rows)}
</table>
</div>

<div class="section">
<h2>Data Plane Metrics</h2>
{'<table><tr><th>Type</th><th>Pair</th><th>Tier</th><th>Result</th><th>Detail</th></tr>' + ''.join(dp_rows) + '</table>' if dp_rows else '<p class="empty">No data plane events</p>'}
</div>

<div class="section">
<h2>Chaos Events ({len(chaos)})</h2>
{'<table><tr><th>Time</th><th>Tier</th><th>Action</th><th>Node</th><th>Params</th></tr>' + ''.join(chaos_rows) + '</table>' if chaos_rows else '<p class="empty">No chaos events</p>'}
</div>

<div class="footer">
    wgmesh integration test observability &middot; {len(events)} trace events
</div>
</body>
</html>"""


def main():
    if len(sys.argv) < 3:
        print(f"Usage: {sys.argv[0]} <trace.jsonl> <output.html>",
              file=sys.stderr)
        sys.exit(1)

    trace_path = sys.argv[1]
    output_path = sys.argv[2]

    if not Path(trace_path).exists():
        print(f"Trace file not found: {trace_path}", file=sys.stderr)
        sys.exit(1)

    events = load_events(trace_path)
    if not events:
        print("Warning: no events found in trace file", file=sys.stderr)

    html = generate_html(events)
    Path(output_path).write_text(html)
    print(f"Report generated: {output_path} ({len(events)} events)")


if __name__ == "__main__":
    main()
