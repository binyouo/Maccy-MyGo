#!/usr/bin/env python3
"""macOS isolated full-app comparison. Uses synthetic data + a named pasteboard.

Original apps are copies with dedicated sandbox identities and an injected
AppKit probe; their product algorithms are unchanged. Never runs bench.sh.
"""
import argparse, base64, ctypes, datetime, hashlib, json, os, pathlib, plistlib
import random, shutil, socket, sqlite3, statistics, struct, subprocess, time, zlib

REPO = pathlib.Path(__file__).resolve().parents[3]
HERE = pathlib.Path(__file__).resolve().parent
ROOT = pathlib.Path('/tmp/maccy-comparison-20261106')
BOARD = 'io.biny.maccy.fullcompare'
LIBPROC = ctypes.CDLL('/usr/lib/libproc.dylib')
class Timebase(ctypes.Structure):
    _fields_=[('numer',ctypes.c_uint32),('denom',ctypes.c_uint32)]
TIMEBASE=Timebase()
ctypes.CDLL('/usr/lib/libSystem.B.dylib').mach_timebase_info(ctypes.byref(TIMEBASE))
NS_PER_TICK=TIMEBASE.numer/TIMEBASE.denom
FIELDS = ['user','system','package_wakes','interrupt_wakes','pageins','wired','resident','footprint','start','exit','child_user','child_system','child_package_wakes','child_interrupt_wakes','child_pageins','child_elapsed','disk_read','disk_written']
class Usage(ctypes.Structure):
    _fields_ = [('uuid',ctypes.c_ubyte*16)] + [(x,ctypes.c_uint64) for x in FIELDS]

def usage(pid):
    data=Usage()
    if LIBPROC.proc_pid_rusage(pid,2,ctypes.byref(data)) != 0: raise OSError('proc_pid_rusage')
    return {key:getattr(data,key) for key in FIELDS}

def cpu(data): return (data['user']+data['system'])*NS_PER_TICK/1e9

def run(command, **kwargs): return subprocess.run([str(x) for x in command],check=True,**kwargs)

def save(name,data):
    path=ROOT/'out'/name;path.parent.mkdir(parents=True,exist_ok=True)
    path.write_text(json.dumps(data,ensure_ascii=False,indent=2))

def png(index,width=3840,height=2160):
    def chunk(kind,data):return struct.pack('>I',len(data))+kind+data+struct.pack('>I',zlib.crc32(kind+data)&0xffffffff)
    raw=b''.join(b'\0'+bytes(((40+index*11)%255,100 if y<height//2 else 190,220 if y<height//2 else 40,255))*width for y in range(height))
    return b'\x89PNG\r\n\x1a\n'+chunk(b'IHDR',struct.pack('>IIBBBBB',width,height,8,6,0,0,0))+chunk(b'IDAT',zlib.compress(raw))+chunk(b'IEND',b'')

class AttachedProcess:
    def __init__(self,pid):self.pid=pid
    def poll(self):
        try:os.kill(self.pid,0);return None
        except ProcessLookupError:return 0
    def wait(self,timeout):
        end=time.perf_counter()+timeout
        while self.poll() is None:
            if time.perf_counter()>end:raise subprocess.TimeoutExpired('app',timeout)
            time.sleep(.05)
    def terminate(self):os.kill(self.pid,15)
    def kill(self):os.kill(self.pid,9)

class App:
    def __init__(self,version):
        self.version=version;self.proc=None;self.sock=None;self.stream=None
        if version=='go':
            self.bundle=ROOT/'MyGo.app';self.exe=self.bundle/'Contents/MacOS/MyGo'
            self.store=pathlib.Path.home()/'Library/Application Support/MaccyCompare20261106'
            self.db=None;self.probe=ROOT/'go.sock'
        else:
            self.bundle=ROOT/('Maccy'+version+'.app');self.exe=self.bundle/'Contents/MacOS/Maccy'
            self.container=pathlib.Path.home()/('Library/Containers/io.biny.macbench'+version)/'Data'
            self.store=self.container/'Library/Application Support/Maccy';self.db=self.store/'Storage.sqlite'
            self.probe=self.container/'tmp/probe.sock'
    def start(self,ui=False):
        self.probe.parent.mkdir(parents=True,exist_ok=True)
        if self.probe.exists():self.probe.unlink()
        env=dict(os.environ,MACCY_BENCH_BOARD=BOARD,MACCY_BENCH_SOCKET=str(self.probe),MACCY_BENCH_EXECUTABLE=str(self.exe),DYLD_INSERT_LIBRARIES=str(self.bundle/'Contents/Frameworks/BenchProbe.dylib'))
        env.pop('MYGO_AUTOMATION',None)
        if self.version=='go':
            env['MACCY_NAME']='MaccyCompare20261106'
            if ui:env.update(MACCY_TEST_SCREEN='2',MACCY_AUTOSHOW='1',MACCY_KEEPOPEN='1')
        if ui:env['MACCY_BENCH_KEEP_OPEN']='1'
        self.log=open(ROOT/'out'/('app-'+self.version+'.log'),'a')
        start=time.perf_counter()
        if ui:
            command=['/usr/bin/open','-n','-g','-a',str(self.bundle),'--stdout',str(ROOT/'out'/('ui-app-'+self.version+'.log')),'--stderr',str(ROOT/'out'/('ui-app-'+self.version+'.log'))]
            for key,value in env.items():
                if key.startswith('MACCY_') or key=='DYLD_INSERT_LIBRARIES':command+=['--env',key+'='+value]
            command+=['--args','-SUEnableAutomaticChecks','NO','-SUAutomaticallyUpdate','NO']
            run(command,capture_output=True)
            end=time.perf_counter()+10
            while True:
                found=[]
                for line in subprocess.check_output(['ps','-axo','pid=,comm='],text=True).splitlines():
                    parts=line.strip().split(None,1)
                    if len(parts)==2 and parts[1].startswith(('/tmp/maccy-comparison','/private/tmp/maccy-comparison')) and pathlib.Path(parts[1]).resolve()==self.exe.resolve():found.append(int(parts[0]))
                if found:self.proc=AttachedProcess(found[-1]);break
                if time.perf_counter()>end:raise TimeoutError('LaunchServices PID')
                time.sleep(.05)
        else:self.proc=subprocess.Popen([str(self.exe),'-SUEnableAutomaticChecks','NO','-SUAutomaticallyUpdate','NO'],env=env,stdout=self.log,stderr=self.log)
        deadline=start+15
        while not self.probe.exists():
            if self.proc.poll() is not None:raise RuntimeError('app exited: '+self.version)
            if time.perf_counter()>deadline:raise TimeoutError('probe socket '+self.version)
            time.sleep(.01)
        self.sock=socket.socket(socket.AF_UNIX);self.sock.settimeout(15);self.sock.connect(str(self.probe));self.stream=self.sock.makefile('rwb')
        while not self.call('state')['launched']:
            if time.perf_counter()>deadline:raise TimeoutError('app launch '+self.version)
            time.sleep(.01)
        return (time.perf_counter()-start)*1000
    def call(self,op,**params):
        self.stream.write((json.dumps(dict(op=op,**params))+'\n').encode());self.stream.flush()
        line=self.stream.readline()
        if not line:raise RuntimeError('probe closed '+self.version)
        result=json.loads(line)
        if result.get('error'):raise RuntimeError(str(result))
        return result
    def stop(self):
        if self.sock:
            try:self.call('quit')
            except Exception:pass
            self.sock.close();self.sock=None
        if self.proc and self.proc.poll() is None:
            try:self.proc.wait(timeout=3)
            except subprocess.TimeoutExpired:
                self.proc.terminate()
                try:self.proc.wait(timeout=3)
                except subprocess.TimeoutExpired:self.proc.kill();self.proc.wait()
        if self.proc:self.log.close();self.proc=None
    def settings(self,**values):
        if self.version!='go':return self.call('set',values=values)
        config=json.loads((self.store/'config.json').read_text());config.update(values)
        # Runtime MyGo settings are driven through its own UI; disk edits here
        # are only used while the app is stopped.
        if self.proc:raise RuntimeError('Go settings require restart')
        (self.store/'config.json').write_text(json.dumps(config))
    def configure(self,count=200,interval=.5):
        common=dict(historySize=count,clipboardCheckInterval=interval,showInStatusBar=False,showSpecialSymbols=False,showApplicationIcons=False,openPreviewAutomatically=False,clearOnQuit=False,clearSystemClipboard=False,showRecentCopyInMenuBar=False,popupScreen=2,popupPosition='center',previewWidth=400)
        if self.version=='go':
            common.update(globalShortcut='',showNotifications=False,windowWidth=450,windowHeight=600,previewWidth=400,searchMode='exact')
            self.store.mkdir(parents=True,exist_ok=True)
            (self.store/'config.json').write_text(json.dumps(common))
        else:
            self.start();self.settings(**common,windowSize='[450,600]',searchMode='exact');self.stop()
    def rows(self):
        if self.version=='go':return json.loads((self.store/'history.json').read_text())['items']
        with sqlite3.connect('file:'+str(self.db)+'?mode=ro',uri=True) as c:
            c.row_factory=sqlite3.Row
            return [dict(title=r['ZTITLE'],copies=r['ZNUMBEROFCOPIES'],pk=r['Z_PK'],pin=r['ZPIN']) for r in c.execute('select * from ZHISTORYITEM')]
    def disk(self):
        paths=list(self.store.rglob('*'))
        result=dict(bytes=sum(p.stat().st_size for p in paths if p.is_file()),files=sum(p.is_file() for p in paths),items=len(self.rows()))
        if self.db:
            with sqlite3.connect('file:'+str(self.db)+'?mode=ro',uri=True) as c:
                for key,sql in [('contents','select count(*) from ZHISTORYITEMCONTENT'),('orphans','select count(*) from ZHISTORYITEMCONTENT where ZITEM is null or ZITEM not in (select Z_PK from ZHISTORYITEM)'),('transactions','select count(*) from ATRANSACTION'),('changes','select count(*) from ACHANGE')]:result[key]=c.execute(sql).fetchone()[0]
        return result
    def seed(self,fixture):
        if self.proc:raise RuntimeError('stop app before seeding')
        if self.version=='go':
            blobs=self.store/'blobs';blobs.mkdir(parents=True,exist_ok=True)
            for p in blobs.iterdir():p.unlink()
            items=[]
            mapping={'public.utf8-plain-text':'text/plain','public.png':'image/png','public.rtf':'text/rtf','public.html':'text/html'}
            for i,entry in enumerate(fixture):
                contents=[]
                for content in entry['contents']:
                    data=pathlib.Path(content['file']).read_bytes();h=hashlib.sha256(data).hexdigest();(blobs/h).write_bytes(data)
                    contents.append(dict(type=mapping.get(content['type'],content['type']),hash=h,size=len(data)))
                stamp=datetime.datetime.fromtimestamp(1700000000-entry.get('age_seconds',i),datetime.timezone.utc)
                if entry.get('local_time'):stamp=stamp.astimezone()
                stamp=stamp.isoformat()
                items.append(dict(id=entry['id'],title=entry['title'],application='io.biny.maccy.fixture',firstCopiedAt=stamp,lastCopiedAt=stamp,numberOfCopies=1,contents=contents))
            (self.store/'history.json').write_text(json.dumps(dict(items=items),ensure_ascii=False))
        else:
            # The schema/model metadata was created by this official binary.
            # Only rows in the explicitly isolated fixture database are reset.
            with sqlite3.connect(self.db) as c:
                for table in ['ZHISTORYITEMCONTENT','ZHISTORYITEM','ACHANGE','ATRANSACTION','ATRANSACTIONSTRING']:c.execute('delete from '+table)
                c.execute('update Z_PRIMARYKEY set Z_MAX=0')
                entities=dict(c.execute('select Z_NAME,Z_ENT from Z_PRIMARYKEY'))
                content_pk=0
                for i,entry in enumerate(fixture,1):
                    timestamp=1700000000-entry.get('age_seconds',i-1)-978307200
                    c.execute('insert into ZHISTORYITEM (Z_PK,Z_ENT,Z_OPT,ZNUMBEROFCOPIES,ZFIRSTCOPIEDAT,ZLASTCOPIEDAT,ZAPPLICATION,ZTITLE) values (?,?,1,1,?,?,?,?)',(i,entities['HistoryItem'],timestamp,timestamp,'io.biny.maccy.fixture',entry['title']))
                    for content in entry['contents']:
                        content_pk+=1
                        c.execute('insert into ZHISTORYITEMCONTENT (Z_PK,Z_ENT,Z_OPT,ZITEM,ZTYPE,ZVALUE) values (?,?,1,?,?,?)',(content_pk,entities['HistoryItemContent'],i,content['type'],pathlib.Path(content['file']).read_bytes()))
                c.execute('update Z_PRIMARYKEY set Z_MAX=? where Z_NAME=?',(len(fixture),'HistoryItem'))
                c.execute('update Z_PRIMARYKEY set Z_MAX=? where Z_NAME=?',(content_pk,'HistoryItemContent'))
                c.commit();c.execute('pragma wal_checkpoint(truncate)');c.execute('vacuum')
    def tree(self):return self.call('state',tree=True).get('nodes',[])
    def labels(self):
        result=[]
        for n in self.tree():
            role=n.get('AXRole') or n.get('role')
            if role in ['AXTextField','AXTextArea']:continue
            for k in ['label','value','title','AXTitle','AXDescription','AXValue']:
                if n.get(k):result.append(str(n[k]))
        return result
    def key(self,name):
        if name=='clear':
            return self.call('clear')
        codes={'down':(125,0xf701),'up':(126,0xf700),'home':(115,0xf729),'end':(119,0xf72b),'enter':(36,13),'escape':(53,27),'clear':(32,117),'preview':(49,32),'pin':(35,112),'delete':(51,127)}
        code,char=codes[name];mods={'clear':['control'],'preview':['control'],'pin':['option'],'delete':['option']}.get(name,[])
        return self.call('key',code=code,character=char,modifiers=mods)
    def open(self):
        if self.version=='go':
            # A real second-instance request opens the production panel. No
            # test hook is added to the Go application itself.
            env=dict(os.environ,MACCY_NAME='MaccyCompare20261106')
            for key in ['DYLD_INSERT_LIBRARIES','MACCY_BENCH_SOCKET','MACCY_BENCH_BOARD','MYGO_AUTOMATION']:env.pop(key,None)
            run([self.exe],env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=10)
        else:
            if not any(w['visible'] and w['frame'][2]>100 for w in self.call('state')['windows']):self.call('open')
    def assert_secondary(self):
        windows=self.call('state')['windows'];visible=[w for w in windows if w['visible'] and w['frame'][2]>100]
        if not visible or any(w['frame'][0]>=0 for w in visible):raise AssertionError('test window not on secondary display: '+str(visible))
        return visible
    def screenshot(self,name):
        window=next(w for w in self.call('state')['windows'] if w['visible'] and w['frame'][2]>100)
        path=ROOT/'out'/name
        path.unlink(missing_ok=True)
        result=subprocess.run(['/usr/sbin/screencapture','-x','-o','-l',str(window['id']),str(path)],capture_output=True,text=True,timeout=8)
        if result.returncode and self.version!='go':
            result=self.call('capture');path.write_bytes(base64.b64decode(result['png']))
        return path.exists()
    def press(self,label):
        self.call('focus');time.sleep(.15)
        result=self.call('press',label=label)
        print('PRESS',self.version,label,result,flush=True)
        return result

def fixtures():
    data=ROOT/'data';data.mkdir(parents=True,exist_ok=True)
    titles=json.loads((data/'titles.json').read_text());raw=json.loads(pathlib.Path('/tmp/maccy-bench/data/corpus.json').read_text())
    out={}
    for count in [200,2000]:
        entries=[]
        for i in range(count):
            p=data/('text-%05d.txt'%i)
            if not p.exists():p.write_text(raw[i])
            entries.append(dict(id='text-'+str(i),title=titles[i],contents=[dict(type='public.utf8-plain-text',file=str(p))]))
        out['text'+str(count)]=entries
    mixed=[dict(x) for x in out['text200']]
    for i in range(20):
        p=data/('image-%02d.png'%i)
        if not p.exists():p.write_bytes(png(i))
        mixed[i]=dict(id='image-'+str(i),title='Comparison image %02d'%i,contents=[dict(type='public.png',file=str(p))])
    # A long multi-paragraph body exercises preview without changing title cost.
    p=data/'long-text.txt';p.write_text('Long paragraph for comparison. 中文预览性能。\n'*8000)
    mixed[20]=dict(id='long-text',title='Comparison long text',contents=[dict(type='public.utf8-plain-text',file=str(p))])
    out['mixed200']=mixed
    save('fixture-manifest.json',{k:dict(items=len(v),payload_bytes=sum(pathlib.Path(c['file']).stat().st_size for e in v for c in e['contents']),sha256=hashlib.sha256(json.dumps(v,sort_keys=True).encode()).hexdigest()) for k,v in out.items()})
    return out

def setup():
    (ROOT/'out').mkdir(parents=True,exist_ok=True)
    (ROOT/'data').mkdir(parents=True,exist_ok=True)
    run(['clang','-O2','-fobjc-arc','-Wno-deprecated-declarations','-dynamiclib','-framework','AppKit','-o',ROOT/'probe.dylib',HERE/'probe.m'])
    for v,source in [('280',ROOT/'download/unpacked/Maccy.app'),('271',pathlib.Path('/Applications/Maccy.app'))]:
        app=ROOT/('Maccy'+v+'.app')
        if not app.exists():run(['ditto',source,app])
        p=app/'Contents/Info.plist';d=plistlib.loads(p.read_bytes());d['CFBundleIdentifier']='io.biny.macbench'+v;d['CFBundleName']='Maccy Compare '+v;p.write_bytes(plistlib.dumps(d))
        shutil.copy2(ROOT/'probe.dylib',app/'Contents/Frameworks/BenchProbe.dylib')
        ent=ROOT/('entitlements'+v+'.plist');ent.write_bytes(plistlib.dumps({'com.apple.security.app-sandbox':True,'com.apple.security.files.user-selected.read-only':True,'com.apple.security.network.server':True}))
        run(['codesign','--force','--deep','--sign','-','--entitlements',ent,app],capture_output=True)
    app=ROOT/'MyGo.app';(app/'Contents/MacOS').mkdir(parents=True,exist_ok=True);(app/'Contents/Frameworks').mkdir(exist_ok=True);(app/'Contents/Resources').mkdir(exist_ok=True)
    run(['go','build','-tags','mygo_noinspector','-trimpath','-ldflags','-s -w','-o',app/'Contents/MacOS/MyGo','.'],cwd=REPO,env=dict(os.environ,CGO_ENABLED='0'))
    shutil.copy2(ROOT/'probe.dylib',app/'Contents/Frameworks/BenchProbe.dylib')
    shutil.copy2(REPO/'resources/ocr',app/'Contents/Resources/ocr')
    shutil.copy2(REPO/'resources/darwin/libmaccy-poll.dylib',app/'Contents/Resources/libmaccy-poll.dylib')
    (app/'Contents/Info.plist').write_bytes(plistlib.dumps(dict(CFBundleIdentifier='io.biny.mygobench',CFBundleExecutable='MyGo',CFBundleName='MyGo Compare',CFBundleShortVersionString='1.1.0',CFBundlePackageType='APPL',LSUIElement=True,NSHighResolutionCapable=True)))
    run(['codesign','--force','--deep','--sign','-',app],capture_output=True)
    run(['swiftc','-O','-o',ROOT/'titles',HERE/'titles.swift'])
    run([ROOT/'titles','/tmp/maccy-bench/data/corpus.json',ROOT/'data/titles.json'])
    manifest={v:dict(binary_sha256=hashlib.sha256(App(v).exe.read_bytes()).hexdigest(),binary_bytes=App(v).exe.stat().st_size,bundle_bytes=sum(p.stat().st_size for p in App(v).bundle.rglob('*') if p.is_file())) for v in ['271','280','go']}
    manifest.update(go_head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=REPO,text=True).strip(),go_dirty=subprocess.check_output(['git','status','--short'],cwd=REPO,text=True),environment=subprocess.check_output(['sw_vers'],text=True),go_version=subprocess.check_output(['go','version'],text=True),source_sha256={str(p.relative_to(REPO)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [REPO/'main.go',*REPO.glob('internal/**/*.go')] if not p.name.endswith('_test.go')})
    save('manifest.json',manifest)
    for v in ['271','280','go']:
        app=App(v)
        try:app.configure()
        finally:app.stop()
    print('SETUP complete',flush=True)

def micro():
    run(['go','build','-o',ROOT/'search-go','./tools/bench/searchgo'],cwd=REPO)
    fuse=pathlib.Path('/tmp/mygomaccy-fuse-reference/Fuse/Classes')
    run(['swiftc','-O','-o',ROOT/'search-swift',REPO/'tools/bench/swiftsearch/main.swift',*fuse.glob('*.swift')])
    for n,reps in [(200,15),(2000,7),(10000,3)]:
        for kind in ['go','swift']:
            path=ROOT/'out'/('%s-search-%d.json'%(kind,n))
            if kind=='go':args=[ROOT/'search-go','-corpus',ROOT/'data/titles.json','-queries','/tmp/maccy-bench/data/queries.json','-n',str(n),'-reps',str(reps),'-warmup','2']
            else:args=[ROOT/'search-swift',ROOT/'data/titles.json','/tmp/maccy-bench/data/queries.json',str(n),str(reps),'2']
            with path.open('w') as output:run(args,stdout=output,stderr=subprocess.DEVNULL)
            print('SEARCH',kind,n,flush=True)

def app_metrics(reps):
    fs=fixtures();result=[]
    for dataset in ['text200','mixed200','text2000']:
        for v in ['271','280','go']:
            app=App(v)
            try:
                app.configure(count=len(fs[dataset]));app.seed(fs[dataset])
                for sample in range(reps):
                    t0=time.perf_counter();ready=app.start();previous=usage(app.proc.pid);at=time.perf_counter();stable=0;deadline=at+12
                    while time.perf_counter()<deadline:
                        time.sleep(.1);now=usage(app.proc.pid);end=time.perf_counter()
                        utilization=(cpu(now)-cpu(previous))/(end-at)*100
                        stable=stable+1 if utilization<2 else 0
                        previous,at=now,end
                        if stable>=3:break
                    quiet=(time.perf_counter()-t0)*1000
                    time.sleep(1);before=usage(app.proc.pid);idle_start=time.perf_counter();time.sleep(8);after=usage(app.proc.pid);duration=time.perf_counter()-idle_start
                    result.append(dict(version=v,dataset=dataset,sample=sample,launch_did_finish_ms=ready,quiet_ms=quiet,quiet_reached=stable>=3,footprint_bytes=after['footprint'],resident_bytes=after['resident'],idle_cpu_percent=(cpu(after)-cpu(before))/duration*100,idle_package_wakes_per_s=(after['package_wakes']-before['package_wakes'])/duration,idle_interrupt_wakes_per_s=(after['interrupt_wakes']-before['interrupt_wakes'])/duration,history_items=len(app.rows()),disk=app.disk()))
                    save('app-metrics.json',result);app.stop();print('APP',v,dataset,sample,round(ready),round(quiet),round(after['footprint']/1048576,1),flush=True)
            finally:app.stop()

def quiet_metrics(reps):
    """Repeat startup with the validated mach-tick CPU conversion; no idle wait."""
    fs=fixtures();result=[]
    for dataset in ['text200','mixed200','text2000']:
        for v in ['271','280','go']:
            a=App(v)
            try:
                a.configure(count=len(fs[dataset]));a.seed(fs[dataset])
                for sample in range(reps):
                    t0=time.perf_counter();ready=a.start();previous=usage(a.proc.pid);at=time.perf_counter();stable=0;trace=[]
                    while time.perf_counter()-t0<12:
                        time.sleep(.1);now=usage(a.proc.pid);end=time.perf_counter();utilization=(cpu(now)-cpu(previous))/(end-at)*100
                        trace.append(dict(elapsed_ms=(end-t0)*1000,cpu_percent=utilization))
                        stable=stable+1 if utilization<2 else 0
                        previous,at=now,end
                        if stable>=3:break
                    result.append(dict(version=v,dataset=dataset,sample=sample,launch_did_finish_ms=ready,quiet_ms=(time.perf_counter()-t0)*1000,quiet_reached=stable>=3,trace=trace,timebase_numer=TIMEBASE.numer,timebase_denom=TIMEBASE.denom))
                    save('quiet-metrics.json',result);a.stop();print('QUIET',v,dataset,sample,round(result[-1]['quiet_ms']),flush=True)
            finally:a.stop()

def wait_for(fn,timeout=5):
    end=time.perf_counter()+timeout
    while time.perf_counter()<end:
        result=fn()
        if result:return result
        time.sleep(.005)
    raise TimeoutError('condition not reached')

def write_marker(app,text,extra=None):
    return app.call('write',items=[[dict(type='public.utf8-plain-text',text=text),*(extra or [])]])

def captures():
    fs=fixtures();apps=[App(v) for v in ['271','280','go']];result={'latency':[],'functional':[]}
    try:
        for app in apps:app.configure();app.seed(fs['text200']);app.start()
        writer=apps[1];rng=random.Random(42)
        for i in range(30):
            time.sleep(rng.uniform(.05,.95));marker='FULLCOMPARE-LAG-%03d'%i;start=time.perf_counter();write_marker(writer,marker);pending={a.version:a for a in apps};seen={}
            deadline=start+5
            while pending and time.perf_counter()<deadline:
                for v,a in list(pending.items()):
                    if any(row['title']==marker for row in a.rows()):seen[v]=(time.perf_counter()-start)*1000;del pending[v]
                time.sleep(.005)
            result['latency'].append(dict(marker=marker,ms=seen,missed=list(pending)))
        marker='FULLCOMPARE-DUPLICATE'
        for _ in range(2):write_marker(writer,marker);time.sleep(.75)
        # A later mutation flushes deferred SwiftData persistence. This is a
        # disk-observation test, not a direct in-memory counter measurement.
        write_marker(writer,'FULLCOMPARE-DUPLICATE-FLUSH');time.sleep(.8)
        for a in apps:
            def duplicate(a=a):
                rows=[r for r in a.rows() if r['title']==marker]
                return next((r for r in rows if r.get('copies',r.get('numberOfCopies'))==2),None)
            try:row=wait_for(duplicate,timeout=10)
            except TimeoutError:row=next(r for r in a.rows() if r['title']==marker)
            copies=row.get('copies',row.get('numberOfCopies'))
            result['functional'].append(dict(test='identical-copy-count',version=a.version,pass_=copies==2,observed=copies))
        for kind in ['org.nspasteboard.ConcealedType','org.nspasteboard.TransientType','org.nspasteboard.AutoGeneratedType']:
            marker='FULLCOMPARE-IGNORED-'+kind;write_marker(writer,marker,[dict(type=kind,text='')]);time.sleep(.7)
            for a in apps:result['functional'].append(dict(test=kind,version=a.version,pass_=not any(r['title']==marker for r in a.rows())))
        marker='  FULLCOMPARE-SPACE-TITLE  ';write_marker(writer,marker);time.sleep(.8)
        for a in apps:result['functional'].append(dict(test='title-trims-edge-spaces',version=a.version,pass_=any(r['title']==marker.strip() for r in a.rows())))
        formats=[('rtf','public.rtf',b'{\\rtf1\\ansi FULLCOMPARE RTF formatted text}'),('html','public.html',b'<html><body><b>FULLCOMPARE HTML formatted text</b></body></html>')]
        for name,typ,data in formats:
            writer.call('write',items=[[dict(type=typ,base64=base64.b64encode(data).decode())]]);time.sleep(1)
            for a in apps:result['functional'].append(dict(test=name+'-only-preview-title',version=a.version,pass_=any(('FULLCOMPARE '+name.upper()) in r['title'] for r in a.rows())))
        marker='FULLCOMPARE-WEBARCHIVE'
        write_marker(writer,marker,[dict(type='com.apple.webarchive',text='synthetic web archive')]);time.sleep(.8)
        for a in apps:
            if a.version=='go':
                row=next(r for r in a.rows() if r['title']==marker)
                types=[c['type'] for c in row['contents']]
            else:
                with sqlite3.connect('file:'+str(a.db)+'?mode=ro',uri=True) as c:
                    types=[r[0] for r in c.execute('select c.ZTYPE from ZHISTORYITEMCONTENT c join ZHISTORYITEM i on i.Z_PK=c.ZITEM where i.ZTITLE=?',(marker,))]
            result['functional'].append(dict(test='webarchive-not-stored',version=a.version,pass_='com.apple.webarchive' not in types,observed_types=types))
        file_url=(ROOT/'data/text-00000.txt').as_uri()
        writer.call('write',items=[[dict(type='public.file-url',text=file_url)]]);time.sleep(.8)
        for a in apps:result['functional'].append(dict(test='file-url-title',version=a.version,pass_=any(r['title']==file_url for r in a.rows())))
        save('capture.json',result);print('CAPTURE latency and basic cases complete',flush=True)
    finally:
        for a in apps:a.stop()

def ui_metrics():
    fs=fixtures();versions=os.environ.get('MACCY_BENCH_UI_VERSIONS','271,280,go').split(',')
    old=ROOT/'out/ui-v2.json'
    result=[r for r in json.loads(old.read_text()) if r['version'] not in versions] if old.exists() else []
    for v in versions:
        a=App(v)
        try:
            a.configure();a.seed(fs['text200']);a.start(ui=True)
            if v!='go':a.open()
            wait_for(lambda:any(w['visible'] and w['frame'][2]>100 for w in a.call('state')['windows']),timeout=8)
            a.assert_secondary();time.sleep(1);a.tree()
            titles={e['title'] for e in fs['text200']};target='needle-alpha-42 unique marker for benchmark queries'
            samples=[]
            for _ in range(10):
                a.call('fill',text='');wait_for(lambda:len(set(a.labels())&titles)>1);time.sleep(.05)
                start=time.perf_counter();a.call('fill',text='needle-alpha-42')
                wait_for(lambda:(set(a.labels())&titles)=={target})
                samples.append((time.perf_counter()-start)*1000)
            result.append(dict(version=v,scenario='exact-query-visible-result',ms=samples,windows=a.assert_secondary(),screenshot=a.screenshot(v+'-search.png')))
            save('ui-v2.json',result);a.stop()
            gallery=[dict(e,age_seconds=i*60,local_time=True) for i,e in enumerate(fs['mixed200'])]
            a.configure()
            if v=='go':a.settings(openPreviewAutomatically=True,previewDelay=200)
            else:
                a.start();a.settings(openPreviewAutomatically=True,previewDelay=200);a.stop()
            a.seed(gallery);a.start(ui=True)
            if v!='go':a.open()
            wait_for(lambda:any(w['visible'] and w['frame'][2]>100 for w in a.call('state')['windows']),timeout=8)
            a.call('focus');time.sleep(.5);a.tree()
            a.call('fill',text='Comparison image 00')
            try:wait_for(lambda:any(w['visible'] and w['frame'][2]>700 for w in a.call('state')['windows']),timeout=6)
            except TimeoutError:
                if any('显示预览' in label for label in a.labels()):a.press('显示预览')
            wait_for(lambda:any(w['visible'] and w['frame'][2]>700 for w in a.call('state')['windows']),timeout=8)
            time.sleep(2);before=usage(a.proc.pid);samples=[]
            for i in range(20):
                a.call('fill',text='Comparison image %02d'%i)
                expected=datetime.datetime.fromtimestamp(1700000000-i*60).strftime('%H:%M')
                wait_for(lambda expected=expected:any(expected in text or expected.lstrip('0') in text for text in a.labels()),timeout=8)
                samples.append(usage(a.proc.pid));time.sleep(.1)
            time.sleep(3);after=usage(a.proc.pid)
            result.append(dict(version=v,scenario='twenty-4k-image-previews',before=before,after=after,samples=samples,windows=a.assert_secondary(),screenshot=a.screenshot(v+'-images.png')))
            a.call('fill',text='Comparison long text');time.sleep(5)
            result.append(dict(version=v,scenario='long-text-preview',usage=usage(a.proc.pid),windows=a.assert_secondary()))
            a.call('close');time.sleep(6)
            result.append(dict(version=v,scenario='closed-after-previews',usage=usage(a.proc.pid)))
            save('ui-v2.json',result);print('UI V2',v,'complete',flush=True)
        except Exception:
            try:save('ui-v2-failure-'+v+'.json',a.call('state',tree=True))
            except Exception:pass
            raise
        finally:a.stop()

def stress_only(count):
    fs=fixtures();apps=[App(v) for v in ['271','280','go']];result={'pace_ms':120,'writes':count,'misses':[],'measurements':[]}
    try:
        for a in apps:a.configure(interval=.05);a.seed(fs['text200']);a.start()
        before={a.version:dict(usage=usage(a.proc.pid),disk=a.disk()) for a in apps};start=time.perf_counter()
        for i in range(count):
            at=time.perf_counter();marker='FULLCOMPARE-PACED-%05d'%i;write_marker(apps[1],marker)
            pending={a.version:a for a in apps};deadline=at+3
            while pending and time.perf_counter()<deadline:
                for v,a in list(pending.items()):
                    if any(r['title']==marker for r in a.rows()):del pending[v]
                time.sleep(.005)
            if pending:result['misses'].append(dict(index=i,versions=list(pending),alive={v:a.proc.poll() for v,a in pending.items()}))
            time.sleep(max(0,.12-(time.perf_counter()-at)))
            if i%100==0:save('stress.json',result);print('PACED',i,'misses',len(result['misses']),flush=True)
        duration=time.perf_counter()-start;time.sleep(8)
        for a in apps:
            after=usage(a.proc.pid);b=before[a.version]['usage']
            result['measurements'].append(dict(version=a.version,duration_s=duration,before=before[a.version],after=dict(usage=after,disk=a.disk()),self_cpu_s=cpu(after)-cpu(b),child_cpu_s=(after['child_user']+after['child_system']-b['child_user']-b['child_system'])*NS_PER_TICK/1e9))
        save('stress.json',result);print('PACED complete',flush=True)
    finally:
        for a in apps:a.stop()

def ocr_warm():
    fs=fixtures();apps=[App(v) for v in ['271','280','go']];result=[]
    try:
        for a in apps:a.configure();a.seed(fs['text200']);a.start()
        payload=base64.b64encode((ROOT/'data/ocr.png').read_bytes()).decode();start=time.perf_counter()
        apps[1].call('write',items=[[dict(type='public.png',base64=payload)]])
        pending={a.version:a for a in apps};deadline=start+30
        while pending and time.perf_counter()<deadline:
            for v,a in list(pending.items()):
                row=next((r for r in a.rows() if 'fullcompare' in r['title'].lower()),None)
                if row:result.append(dict(version=v,title=row['title'],ms=(time.perf_counter()-start)*1000));del pending[v]
            time.sleep(.01)
        for v in pending:result.append(dict(version=v,timeout=True))
        save('ocr-warm.json',result);print('OCR',result,flush=True)
    finally:
        for a in apps:a.stop()

def idle_metrics():
    """Six small isolated apps; no visible windows and only the private board."""
    run(['go','build','-tags','mygo_noinspector','-trimpath','-ldflags','-s -w','-o',ROOT/'idleprobe','./tools/bench/fullcompare/idleprobe'],cwd=REPO,env=dict(os.environ,CGO_ENABLED='0'))
    shutil.copy2(REPO/'resources/darwin/libmaccy-poll.dylib',ROOT/'libmaccy-poll.dylib')
    results=[]
    for mode in ['empty','hidden','watcher','timer','mainpoll','count']:
        probe=ROOT/('idle-'+mode+'.sock')
        probe.unlink(missing_ok=True)
        env=dict(os.environ,MACCY_BENCH_BOARD=BOARD,MACCY_BENCH_SOCKET=str(probe),MACCY_BENCH_EXECUTABLE=str(ROOT/'idleprobe'),DYLD_INSERT_LIBRARIES=str(ROOT/'probe.dylib'))
        with (ROOT/'out'/('idle-'+mode+'.log')).open('w') as log:
            proc=subprocess.Popen([str(ROOT/'idleprobe'),mode],env=env,stdout=log,stderr=log)
            try:
                time.sleep(2);before=usage(proc.pid);start=time.perf_counter();time.sleep(8);after=usage(proc.pid);elapsed=time.perf_counter()-start
                results.append(dict(mode=mode,cpu_percent=(cpu(after)-cpu(before))/elapsed*100,interrupt_wakes_per_s=(after['interrupt_wakes']-before['interrupt_wakes'])/elapsed,package_wakes_per_s=(after['package_wakes']-before['package_wakes'])/elapsed))
            finally:
                if proc.poll() is None:proc.terminate();proc.wait(timeout=5)
        save('idle-ablation.json',results)

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('phase',choices=['setup','micro','apps','quiet','capture','stress','ocr','ui','idle','all']);parser.add_argument('--reps',type=int,default=5);parser.add_argument('--stress',type=int,default=500)
    args=parser.parse_args();(ROOT/'out').mkdir(parents=True,exist_ok=True)
    try:
        for phase in (['setup','micro','apps','quiet','capture','stress','idle'] if args.phase=='all' else [args.phase]):
            {'setup':setup,'micro':micro,'apps':lambda:app_metrics(args.reps),'quiet':lambda:quiet_metrics(args.reps),'capture':captures,'stress':lambda:stress_only(args.stress),'ocr':ocr_warm,'ui':ui_metrics,'idle':idle_metrics}[phase]()
    except KeyboardInterrupt:raise SystemExit(130)
