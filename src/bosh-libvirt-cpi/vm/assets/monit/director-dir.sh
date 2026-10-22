python3 -c "
import http.server, socketserver, sys, time, subprocess, os
socketserver.TCPServer.allow_reuse_address = True
def xml():
  inc = str(int(time.time()))
  import glob, re, json
  svcs = []
  for f in sorted(glob.glob('/var/vcap/monit/job/*.monitrc')):
    for m in re.findall(r'check process (\S+)', open(f).read()):
      if m not in svcs: svcs.append(m)
  if not svcs:
    try:
      spec = json.load(open('/var/vcap/bosh/spec.json'))
      for tpl in spec.get('job',{}).get('templates',[]):
        n = tpl.get('name','')
        if n and n not in ('bpm','libvirt_cpi') and n not in svcs: svcs.append(n)
    except: pass
  svc_xml = ''.join('<service name=\"%(s)s\" type=\"5\"><status>0</status><monitor>1</monitor><pendingaction>0</pendingaction></service>' % {'s':s} for s in svcs)
  grp_xml = '<servicegroup name=\"vcap\">' + ''.join('<service>%s</service>' % s for s in svcs) + '</servicegroup>' if svcs else ''
  result = ('<monit id=\"stub\" incarnation=\"'+ inc + '\" version=\"5\"><services>' + svc_xml + '</services><servicegroups>' + grp_xml + '</servicegroups></monit>')
  import os as _os
  try: _files = _os.listdir('/var/vcap/monit/job')
  except: _files = []
  open('/tmp/monit-svcs.log','a').write(inc+' svcs='+str(svcs)+' files='+str(_files)+'\n')
  return result.encode()
def load_bpm(path):
  import re as _r
  text = open(path).read()
  procs = []; cur = None; in_args = False; in_env = False; in_unsafe = False
  for line in text.splitlines():
    if _r.match(r'^\s*-\s+name:', line):
      if cur: procs.append(cur)
      _q = chr(34); cur = {'name': line.split('name:',1)[1].strip().strip(_q+chr(39)), 'executable': '', 'args': [], 'env': {}, 'privileged': False}
      in_args = False; in_env = False; in_unsafe = False
    elif cur is None: continue
    elif _r.match(r'\s+executable:', line):
      _q = chr(34); cur['executable'] = line.split('executable:',1)[1].strip().strip(_q+chr(39)); in_args = False; in_env = False; in_unsafe = False
    elif _r.match(r'\s+args:', line):
      in_args = True; in_env = False; in_unsafe = False
      inline = line.split('args:',1)[1].strip()
      if inline.startswith('['):
        _q = chr(34); cur['args'] = [a.strip().strip(_q+chr(39)) for a in inline.strip('[]').split(',') if a.strip()]; in_args = False
    elif _r.match(r'\s+env:', line): in_env = True; in_args = False; in_unsafe = False
    elif _r.match(r'\s+unsafe:', line): in_unsafe = True; in_args = False; in_env = False
    elif in_unsafe and _r.match(r'\s+privileged:\s+true', line): cur['privileged'] = True
    elif in_args and _r.match(r'\s+-\s+', line):
      _q = chr(34); cur['args'].append(line.split('-',1)[1].strip().strip(_q+chr(39)))
    elif in_env and ':' in line and _r.match(r'\s+\w', line):
      _q = chr(34); k,v = line.split(':',1); cur['env'][k.strip()] = v.strip().strip(_q+chr(39))
    elif _r.match(r'\s+(executable|name|args|env|unsafe):', line): in_args = False; in_env = False; in_unsafe = False
  if cur: procs.append(cur)
  return {'processes': procs}
def _pid_alive(pidfile):
  try:
    pid = int(open(pidfile).read().strip())
    os.kill(pid, 0)
    return True
  except: return False
def start_svc(svc):
  import glob, socket, time
  import os as _os2; _os2.makedirs('/var/vcap/bosh/log', exist_ok=True)
  # Map monit service names that live under the director job to their bpm process names.
  # director_* and metrics_server are all bpm processes inside the director job.
  DIRECTOR_BPM_MAP = {
    'director': 'director',
    'director_nginx': 'nginx',
    'director_scheduler': 'scheduler',
    'director_sync_dns': 'sync_dns',
    'metrics_server': 'metrics_server',
  }
  # Services that need postgres to be ready before they can start
  NEEDS_POSTGRES = ('director', 'director_scheduler')
  is_director_svc = (svc in DIRECTOR_BPM_MAP or svc.startswith('worker_') or svc.startswith('dynamic_disks_worker_'))
  needs_pg = (svc in NEEDS_POSTGRES or svc.startswith('worker_') or svc.startswith('dynamic_disks_worker_'))
  if is_director_svc:
    import threading
    def _start_async():
      log = open('/var/vcap/bosh/log/monit-'+svc+'.log','a')
      _main_pf = '/var/vcap/sys/run/bpm/director/'+DIRECTOR_BPM_MAP.get(svc,svc)+'.pid'
      if _pid_alive(_main_pf):
        log.write('start_svc: '+svc+' already running, skipping\n'); log.flush()
        return
      if needs_pg:
        pg_host = '127.0.0.1'
        log.write('waiting for postgres on '+pg_host+':5432\n'); log.flush()
        for i in range(5400):
          try:
            s = socket.create_connection((pg_host, 5432), 1); s.close()
            import subprocess as _sp
            r = _sp.run(['/var/vcap/packages/postgres-15/bin/pg_isready','-h',pg_host,'-p','5432'],
              capture_output=True, timeout=5)
            log.write('pg_isready rc='+str(r.returncode)+' out='+r.stdout.decode()[:60]+'\n'); log.flush()
            if r.returncode == 0: break
          except Exception as e:
            if i % 30 == 0: log.write('pg wait err (i='+str(i)+'): '+str(e)+'\n'); log.flush()
          time.sleep(2)
        else:
          log.write('postgres never ready\n'); log.flush(); return
        log.write('postgres ready, starting '+svc+'\n'); log.flush()
      # Run pre-start from the director job
      pre_start = '/var/vcap/jobs/director/bin/pre-start'
      if os.path.exists(pre_start):
        try:
          r = subprocess.run([pre_start], capture_output=True, timeout=120)
          log.write('pre-start rc='+str(r.returncode)+' '+r.stdout.decode()[:200]+r.stderr.decode()[:200]+'\n'); log.flush()
        except Exception as e: log.write('pre-start failed: '+str(e)+'\n'); log.flush()
      # Pre-create runtime-write dirs for the director job
      for _rtdir in ['/var/vcap/data/director/tmp', '/var/vcap/sys/log/director', '/var/vcap/sys/run/director']:
        os.makedirs(_rtdir, exist_ok=True)
        try: os.chown(_rtdir, 1000, 1000)
        except: pass
      # worker_N: use worker_ctl (use_bpm_for_workers=false is the default)
      if svc.startswith('worker_') or svc.startswith('dynamic_disks_worker_'):
        worker_ctl = '/var/vcap/jobs/director/bin/worker_ctl'
        if os.path.exists(worker_ctl):
          log_dir = '/var/vcap/sys/log/director'
          os.makedirs(log_dir, exist_ok=True)
          try: os.chown(log_dir, 1000, 1000)
          except: pass
          data_dir = '/var/vcap/data/director/tmp'
          os.makedirs(data_dir, exist_ok=True)
          try: os.chown(data_dir, 1000, 1000)
          except: pass
          try:
            p = subprocess.Popen([worker_ctl, svc, 'start'],
              stdout=log, stderr=log, start_new_session=True)
            log.write('started '+svc+' via worker_ctl pid='+str(p.pid)+'\n'); log.flush()
          except Exception as e: log.write('worker_ctl failed: '+str(e)+'\n'); log.flush()
          return
      # director bpm services: look up by mapped process name
      bpm_proc_name = DIRECTOR_BPM_MAP.get(svc, svc)
      bpmyml = '/var/vcap/jobs/director/config/bpm.yml'
      if not os.path.exists(bpmyml): return
      try:
        cfg = load_bpm(bpmyml)
        procs = [p for p in cfg.get('processes',[]) if p.get('name','') == bpm_proc_name]
        if not procs: log.write('no bpm process named '+bpm_proc_name+'\n'); log.flush(); return
        setpriv_bin = next((p for p in ['/usr/bin/setpriv','/usr/sbin/setpriv','/sbin/setpriv'] if os.path.exists(p)), None)
        if not setpriv_bin: return
        for proc in procs:
          pname = proc.get('name', svc)
          exe = proc.get('executable','')
          if not exe or not os.path.exists(exe): log.write('skip '+pname+': exe not found\n'); log.flush(); continue
          args = [exe] + proc.get('args',[])
          env2 = dict(os.environ); env2.update(proc.get('env',{}))
          if 'nginx' in exe or 'nginx' in pname:
            for d in ['/var/vcap/data/director/tmp/client_body','/var/vcap/data/director/tmp/proxy','/var/vcap/data/director/tmp/fastcgi','/var/vcap/data/director/tmp/uwsgi','/var/vcap/data/director/tmp/scgi','/var/vcap/sys/log/director']:
              os.makedirs(d, exist_ok=True)
              try: os.chown(d, 1000, 1000)
              except: pass
            ng_log_dir = '/var/vcap/packages/nginx/logs'
            if not os.path.exists(ng_log_dir): os.makedirs(ng_log_dir, exist_ok=True)
            ng_log = ng_log_dir + '/error.log'
            if not os.path.exists(ng_log): open(ng_log,'a').close()
            try: os.chmod(ng_log, 0o666)
            except: pass
          args = [setpriv_bin,'--reuid=1000','--regid=1000','--clear-groups','--'] + args
          pf = '/var/vcap/sys/run/bpm/director/'+pname+'.pid'
          os.makedirs(os.path.dirname(pf), exist_ok=True)
          try: os.chown(os.path.dirname(pf), 1000, 1000)
          except: pass
          p = subprocess.Popen(args, env=env2, stdout=log, stderr=log, start_new_session=True)
          open(pf,'w').write(str(p.pid))
          log.write('started '+pname+' pid='+str(p.pid)+'\n'); log.flush()
          time.sleep(1)
      except Exception as e: log.write('start failed: '+str(e)+'\n'); log.flush()
    threading.Thread(target=_start_async, daemon=True).start()
    return
  log = open('/var/vcap/bosh/log/monit-'+svc+'.log','a')
  # Resolve job directory: monitrc service names like blobstore_nginx live in
  # the 'blobstore' job dir. Scan monitrc files to find which job owns this svc.
  job = svc
  if not os.path.isdir('/var/vcap/jobs/'+svc):
    import re as _re2
    for _mf in sorted(glob.glob('/var/vcap/monit/job/*.monitrc')):
      if _re2.search(r'check process '+_re2.escape(svc)+r'\b', open(_mf).read()):
        _jname = _re2.sub(r'^\d+_','',os.path.basename(_mf).replace('.monitrc',''))
        if os.path.isdir('/var/vcap/jobs/'+_jname): job = _jname; break
  log.write('start_svc svc='+svc+' job='+job+'\n'); log.flush()
  # Run pre-start script as root so it can create required directories
  pre_start = '/var/vcap/jobs/' + job + '/bin/pre-start'
  if os.path.exists(pre_start):
    try:
      r = subprocess.run([pre_start], capture_output=True, timeout=120)
      log.write('pre-start rc='+str(r.returncode)+' '+r.stdout.decode()[:200]+r.stderr.decode()[:200]+'\n'); log.flush()
    except Exception as e: log.write('pre-start failed: '+str(e)+'\n'); log.flush()
  # chown all data/log dirs created by pre-start to vcap (uid 1000)
  # Always mkdir so dirs exist in tmpfs (bosh-agent mounts tmpfs over /var/vcap/sys/log)
  import glob as _glob
  _chown_roots = ['/var/vcap/data/'+svc, '/var/vcap/sys/log/'+svc, '/var/vcap/sys/run/'+svc] + _glob.glob('/var/vcap/store/'+svc+'*')
  if job == 'blobstore' or svc == 'blobstore' or 'blobstore' in svc:
    _chown_roots.append('/var/vcap/data/blobs')
  for chown_root in _chown_roots:
    os.makedirs(chown_root, exist_ok=True)
    for dirpath, dirnames, filenames in os.walk(chown_root):
      try: os.chown(dirpath, 1000, 1000)
      except: pass
      for f in filenames:
        try: os.chown(os.path.join(dirpath, f), 1000, 1000)
        except: pass
  # For postgres and other services: start all processes in bpm.yml
  bpmyml = '/var/vcap/jobs/' + job + '/config/bpm.yml'
  if os.path.exists(bpmyml):
    try:
      cfg = load_bpm(bpmyml)
      procs = cfg.get('processes',[])
      setpriv_bin = next((p for p in ['/usr/bin/setpriv','/usr/sbin/setpriv','/sbin/setpriv'] if os.path.exists(p)), None)
      if not setpriv_bin: raise FileNotFoundError('setpriv not found')
      for proc in procs:
        pname = proc.get('name', svc)
        exe = proc.get('executable','')
        if not exe: continue
        args = [exe] + proc.get('args',[])
        env = dict(os.environ); env.update(proc.get('env',{}))
        if 'nginx' in exe or 'nginx' in pname:
          for d in ['/var/vcap/data/'+job+'/tmp','/var/vcap/packages/nginx/logs']:
            os.makedirs(d, exist_ok=True)
            try: os.chown(d, 1000, 1000)
            except: pass
          ng_log = '/var/vcap/packages/nginx/logs/error.log'
          if not os.path.exists(ng_log): open(ng_log,'a').close()
          try: os.chmod(ng_log, 0o666)
          except: pass
        if not proc.get('privileged', False):
          args = [setpriv_bin,'--reuid=1000','--regid=1000','--clear-groups','--'] + args
        pf = '/var/vcap/sys/run/bpm/'+svc+'/'+pname+'.pid'
        os.makedirs(os.path.dirname(pf), exist_ok=True)
        try: os.chown(os.path.dirname(pf), 1000, 1000)
        except: pass
        log.write('starting '+pname+' privileged='+str(proc.get('privileged',False))+' exe='+exe+'\n'); log.flush()
        if _pid_alive(pf):
          log.write('skipping '+pname+' already running\n'); log.flush()
          continue
        if pname == 'bosh_nats_sync':
          import fcntl as _fcntl
          _lf = open('/var/vcap/sys/run/bpm/nats/bosh_nats_sync.lock','w')
          try: _fcntl.flock(_lf, _fcntl.LOCK_EX|_fcntl.LOCK_NB)
          except OSError:
            log.write('bosh_nats_sync already running, skipping duplicate\n'); log.flush()
            _lf.close(); continue
          p = subprocess.Popen(args, env=env, stdout=log, stderr=log, start_new_session=True)
          open(pf,'w').write(str(p.pid))
          log.write('started bosh_nats_sync pid='+str(p.pid)+'\n'); log.flush()
          def _watch_nats(proc=p, lf=_lf):
            proc.wait()
            try: _fcntl.flock(lf, _fcntl.LOCK_UN)
            except: pass
            lf.close()
          import threading as _threading
          _threading.Thread(target=_watch_nats, daemon=True).start()
          continue
        p = subprocess.Popen(args, env=env, stdout=log, stderr=log, start_new_session=True)
        open(pf,'w').write(str(p.pid))
        log.write('started '+pname+' pid='+str(p.pid)+'\n'); log.flush()
        if svc == 'postgres' and pname == svc:
          _pg_args = args; _pg_env = env; _pg_host = '127.0.0.1'
          def run_createdb_and_watch():
            for _ in range(60):
              try:
                s = socket.create_connection((_pg_host, 5432), 1); s.close(); break
              except: time.sleep(2)
            import glob
            for f in glob.glob('/var/vcap/jobs/*/bin/create-database'):
              subprocess.run([f], stdout=log, stderr=log, timeout=60)
            while True:
              time.sleep(5)
              try:
                s = socket.create_connection((_pg_host, 5432), 1); s.close()
              except:
                log.write('postgres down - restarting\n'); log.flush()
                try:
                  np = subprocess.Popen(_pg_args, env=_pg_env, stdout=log, stderr=log, start_new_session=True)
                  open(pf,'w').write(str(np.pid)); time.sleep(3)
                except Exception as re: log.write('restart failed: '+str(re)+'\n')
          import threading; threading.Thread(target=run_createdb_and_watch, daemon=True).start()
      return
    except Exception as e: log.write('bpm.yml start failed: '+str(e)+'\n'); log.flush()
  ctl = '/var/vcap/jobs/' + job + '/bin/ctl'
  if os.path.exists(ctl):
    subprocess.Popen([ctl,'start'], stdout=log, stderr=log)
class H(http.server.BaseHTTPRequestHandler):
  def do_GET(self):
    body = xml()
    self.send_response(200)
    self.send_header('Content-Type','text/xml')
    self.send_header('Content-Length', str(len(body)))
    self.end_headers()
    self.wfile.write(body)
  def do_POST(self):
    length = int(self.headers.get('Content-Length','0'))
    body = self.rfile.read(length).decode()
    svc = self.path.strip('/')
    if svc and 'action=start' in body:
      start_svc(svc)
    self.send_response(200)
    self.send_header('Content-Length','0')
    self.end_headers()
  def log_message(self, fmt, *a):
    open('/var/vcap/bosh/log/monit-req.log','a').write('[%s] %s %s\n' % (self.log_date_time_string(), self.command, self.path))
try:
  srv = socketserver.TCPServer(('127.0.0.1',2822),H)
  sys.stderr.write('monit stub: ready\\n')
  sys.stderr.flush()
  srv.serve_forever()
except Exception as e:
  sys.stderr.write('monit stub error: %s\\n' % str(e))
  sys.stderr.flush()
" >/tmp/monit-stub.log 2>&1 &
for i in $(seq 1 30); do
  python3 -c 'import socket,sys; s=socket.socket(); s.settimeout(0.2); s.connect(("127.0.0.1",2822)); s.close()' 2>/dev/null && break
  sleep 0.2
done
