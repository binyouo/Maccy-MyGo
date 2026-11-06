#!/usr/bin/env python3
"""Interleaved before/after idle sampling using the fullcompare isolation.

Prepare the original QA Go bundle at /tmp/maccy-wakeopt-20261106/MyGo-before.app
before replacing the QA binary. This never launches the installed user app.
"""
import hashlib, json, pathlib, statistics, time
import run as bench

OUT = pathlib.Path('/tmp/maccy-wakeopt-20261106')

def idle(reps=5):
    baseline = OUT/'MyGo-before.app'
    if not baseline.exists(): raise SystemExit('missing pre-change QA bundle: '+str(baseline))
    fixtures = bench.fixtures(); results=[]
    app=bench.App('go');app.configure();app.seed(fixtures['text200'])
    for sample in range(reps):
        for variant in (['before','after'] if sample%2==0 else ['after','before']):
            app=bench.App('go')
            if variant=='before':
                app.bundle=baseline;app.exe=baseline/'Contents/MacOS/MyGo'
            try:
                ready=app.start();time.sleep(3)
                before=bench.usage(app.proc.pid);start=time.perf_counter();time.sleep(8)
                after=bench.usage(app.proc.pid);duration=time.perf_counter()-start
                row=dict(variant=variant,sample=sample,ready_ms=ready,duration_s=duration,before=before,after=after,
                    cpu_percent=(bench.cpu(after)-bench.cpu(before))/duration*100,
                    interrupt_wakes_s=(after['interrupt_wakes']-before['interrupt_wakes'])/duration,
                    package_wakes_s=(after['package_wakes']-before['package_wakes'])/duration,
                    footprint_mib=after['footprint']/2**20)
                results.append(row);(OUT/'idle.json').write_text(json.dumps(results,indent=2));print(row['variant'],sample,{k:round(row[k],3) for k in ['cpu_percent','interrupt_wakes_s','package_wakes_s','footprint_mib']},flush=True)
            finally:app.stop()
    summary={v:{k:statistics.median(r[k] for r in results if r['variant']==v) for k in ['cpu_percent','interrupt_wakes_s','package_wakes_s','footprint_mib']} for v in ['before','after']}
    (OUT/'idle-summary.json').write_text(json.dumps(summary,indent=2));print(summary,flush=True)

if __name__=='__main__': idle()
