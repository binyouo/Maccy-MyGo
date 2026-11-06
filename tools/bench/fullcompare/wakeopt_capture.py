import pathlib, json, time, math, statistics
import run as b
out=pathlib.Path('/tmp/maccy-wakeopt-20261106');result={'functional':[],'latency':[],'stress':{}}
fs=b.fixtures();a=b.App('go');a.bundle=out/'Release-QA.app';a.exe=a.bundle/'Contents/MacOS/Maccy MyGo'
def write(text):b.write_marker(a,text)
def seen(text):return next((r for r in a.rows() if r['title']==text),None)
def record(name,condition):
 if not condition:raise AssertionError(name)
 result['functional'].append({'name':name,'pass':True})
def start(**settings):
 a.configure();a.settings(**settings);a.seed([]);a.start();time.sleep(.2)
try:
 start()
 record('startup-does-not-import-existing-board',len(a.rows())==0)
 for _ in range(2):write('WAKEOPT-identical');b.wait_for(lambda:seen('WAKEOPT-identical'),timeout=2);time.sleep(.55)
 record('identical-copy-count',seen('WAKEOPT-identical')['numberOfCopies']==2)
 a.stop();start(ignoreEvents=True,ignoreOnlyNextEvent=True)
 write('WAKEOPT-ignore-next');time.sleep(.7)
 record('one-copy-pause-skips',not seen('WAKEOPT-ignore-next'))
 write('WAKEOPT-ignore-next');b.wait_for(lambda:seen('WAKEOPT-ignore-next'),timeout=2)
 record('one-copy-pause-resumes-identical-bytes',seen('WAKEOPT-ignore-next')['numberOfCopies']==1)
 a.stop();start(ignoreEvents=True)
 write('WAKEOPT-permanent-pause');time.sleep(.7)
 record('permanent-pause-skips',len(a.rows())==0)
 a.stop();a.settings(ignoreEvents=False);a.start();time.sleep(.3)
 record('restart-does-not-import-paused-board',len(a.rows())==0)
 write('WAKEOPT-permanent-pause');b.wait_for(lambda:seen('WAKEOPT-permanent-pause'),timeout=2)
 record('resume-after-restart-captures-next-identical-write',bool(seen('WAKEOPT-permanent-pause')))
 a.stop();a.configure();a.seed(fs['text200']);a.start();time.sleep(1)
 import random
 rng=random.Random(42)
 for i in range(30):
  time.sleep(rng.uniform(.05,.95));marker=f'WAKEOPT-lag-{i}';t=time.perf_counter();write(marker);b.wait_for(lambda:seen(marker),timeout=2)
  result['latency'].append((time.perf_counter()-t)*1000)
 a.stop();a.configure(interval=.05);a.seed(fs['text200']);a.start();time.sleep(2)
 before=b.usage(a.proc.pid);at=time.perf_counter();misses=[]
 for i in range(500):
  t=time.perf_counter();marker=f'WAKEOPT-paced-{i}';write(marker)
  try:b.wait_for(lambda:seen(marker),timeout=3)
  except TimeoutError:misses.append(i)
  time.sleep(max(0,.12-(time.perf_counter()-t)))
  if i%100==0:print('STRESS',i,'misses',len(misses),flush=True)
 elapsed=time.perf_counter()-at;time.sleep(8);after=b.usage(a.proc.pid)
 result['stress']=dict(writes=500,pace_ms=120,interval_ms=50,misses=misses,duration_s=elapsed,self_cpu_s=b.cpu(after)-b.cpu(before),child_cpu_s=(after['child_user']+after['child_system']-before['child_user']-before['child_system'])*b.NS_PER_TICK/1e9,footprint_mib=after['footprint']/2**20)
 record('paced-500-no-misses',not misses)
finally:
 a.stop();(out/'capture-check.json').write_text(json.dumps(result,ensure_ascii=False,indent=2))
print(result,flush=True)
