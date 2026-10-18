python3 -c "
import http.server, socketserver, sys, time, subprocess, os, threading, glob, re
socketserver.TCPServer.allow_reuse_address = True
def xml():
  inc = str(int(time.time()))
  svcs = []
  for f in sorted(glob.glob('/var/vcap/monit/job/*.monitrc')):
    for m in re.findall(r'check process (\S+)', open(f).read()):
      if m not in svcs: svcs.append(m)
  svc_xml = ''.join('<service name=\"%(s)s\" type=\"5\"><status>0</status><monitor>1</monitor><pendingaction>0</pendingaction></service>' % {'s':s} for s in svcs)
  grp_xml = '<servicegroup name=\"vcap\">' + ''.join('<service>%s</service>' % s for s in svcs) + '</servicegroup>' if svcs else ''
  return ('<monit id=\"stub\" incarnation=\"'+inc+'\" version=\"5\"><services>'+svc_xml+'</services><servicegroups>'+grp_xml+'</servicegroups></monit>').encode()
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
  log = open('/var/vcap/bosh/log/monit-'+svc+'.log','a')
  def _do_start():
    job = svc
    if not os.path.isdir('/var/vcap/jobs/'+svc):
      for _mf in sorted(glob.glob('/var/vcap/monit/job/*.monitrc')):
        if re.search(r'check process '+re.escape(svc)+r'\b', open(_mf).read()):
          _jname = re.sub(r'^\d+_','',os.path.basename(_mf).replace('.monitrc',''))
          if os.path.isdir('/var/vcap/jobs/'+_jname): job = _jname; break
    log.write('deploy_start_svc svc='+svc+' job='+job+'\n'); log.flush()
    for _d in ['/var/vcap/data/'+svc, '/var/vcap/sys/log/'+svc, '/var/vcap/sys/run/'+svc]:
      os.makedirs(_d, exist_ok=True)
      try: os.chown(_d, 1000, 1000)
      except: pass
    pre_start = '/var/vcap/jobs/'+job+'/bin/pre-start'
    if os.path.exists(pre_start):
      try:
        r = subprocess.run([pre_start], capture_output=True, timeout=8)
        log.write('pre-start rc='+str(r.returncode)+' '+r.stdout.decode()[:200]+r.stderr.decode()[:200]+'\n'); log.flush()
      except Exception as e: log.write('pre-start timeout/err: '+str(e)+'\n'); log.flush()
    bpmyml = '/var/vcap/jobs/'+job+'/config/bpm.yml'
    if not os.path.exists(bpmyml): return
    try:
      cfg = load_bpm(bpmyml)
      setpriv_bin = next((p for p in ['/usr/bin/setpriv','/usr/sbin/setpriv','/sbin/setpriv'] if os.path.exists(p)), None)
      for proc in cfg.get('processes',[]):
        pname = proc.get('name',svc)
        exe = proc.get('executable','')
        if not exe or not os.path.exists(exe): log.write('skip '+pname+': exe not found\n'); log.flush(); continue
        # nats-server needs auth.json (included by nats.cfg) to exist before it
        # starts. bosh_nats_sync writes the real auth.json then sends --signal reload.
        # Pre-create a valid-but-empty auth.json so nats-server starts, then
        # bosh_nats_sync will overwrite it and reload the running server.
        if svc == 'nats' and pname == 'nats':
          for _nd in ['/var/vcap/data/nats','/var/vcap/sys/log/nats','/var/vcap/sys/run/bpm/nats']:
            os.makedirs(_nd, exist_ok=True)
          _auth = '/var/vcap/data/nats/auth.json'
          if not os.path.exists(_auth):
            open(_auth,'w').close()
            log.write('pre-created empty auth.json for nats-server startup\n'); log.flush()
        args = [exe] + proc.get('args',[])
        env = dict(os.environ); env.update(proc.get('env',{}))
        if 'nginx' in exe or 'nginx' in pname:
          for d in ['/var/vcap/data/'+svc+'/tmp','/var/vcap/packages/nginx/logs']:
            os.makedirs(d, exist_ok=True)
            try: os.chown(d, 1000, 1000)
            except: pass
          ng_log = '/var/vcap/packages/nginx/logs/error.log'
          if not os.path.exists(ng_log): open(ng_log,'a').close()
          try: os.chmod(ng_log, 0o666)
          except: pass
        if not proc.get('privileged', False) and setpriv_bin:
          args = [setpriv_bin,'--reuid=1000','--regid=1000','--clear-groups','--'] + args
        pf = '/var/vcap/sys/run/bpm/'+svc+'/'+pname+'.pid'
        os.makedirs(os.path.dirname(pf), exist_ok=True)
        try: os.chown(os.path.dirname(pf), 1000, 1000)
        except: pass
        log.write('starting '+pname+' exe='+exe+'\n'); log.flush()
        p = subprocess.Popen(args, env=env, stdout=log, stderr=log, start_new_session=True)
        open(pf,'w').write(str(p.pid))
        log.write('started '+pname+' pid='+str(p.pid)+'\n'); log.flush()
    except Exception as e: log.write('bpm start failed: '+str(e)+'\n'); log.flush()
  threading.Thread(target=_do_start, daemon=True).start()
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
