#!/usr/bin/env bash
# LXC E2E failure diagnostics. Invoked as: e2e-debug-lxc.sh <phase>
# phase: wait | worker | upload | deploy | verify
# Reads: DIRECTOR_IP, STORE_DIR, SSH_KEY, BOSH_ENV_ALIAS (exported by e2e.yml)
set +e  # diagnostics must never abort; every probe is best-effort
PHASE="${1:-unknown}"
: "${STORE_DIR:=/tmp/bosh_libvirt_cpi}"
: "${SSH_KEY:=/tmp/bosh-libvirt-ssh-key}"

case "$PHASE" in
  wait)
    echo "=== bosh create-env log (last 80 lines) ===" && tail -80 /tmp/bosh-create-env.log || true
    echo "=== LXC container state ===" && sudo virsh -c lxc:/// list --all 2>/dev/null || true
    VMDIR=$(sudo find ${STORE_DIR}/vms -maxdepth 1 -mindepth 1 -type d 2>/dev/null | head -1)
    echo "VMDIR=$VMDIR"
    [ -n "$VMDIR" ] && echo "=== rootfs dir listing ===" && sudo ls -la "$VMDIR/rootfs/" 2>/dev/null | head -20 || echo "(no rootfs)"
    [ -n "$VMDIR" ] && echo "=== /bosh-lxc-init content ===" && sudo cat "$VMDIR/rootfs/bosh-lxc-init" 2>/dev/null | head -10 || echo "(no bosh-lxc-init)"
    [ -n "$VMDIR" ] && echo "=== bosh-agent-init.log (all) ===" && sudo cat "$VMDIR/rootfs/var/vcap/bosh/log/bosh-agent-init.log" 2>/dev/null || echo "(no bosh-agent-init.log)"
    [ -n "$VMDIR" ] && echo "=== bosh-agent current log ===" && sudo cat "$VMDIR/rootfs/var/vcap/bosh/log/current" 2>/dev/null | grep -v "unlimitedRetry\|DelayedAudit" | tail -30 || echo "(no current log)"
    [ -n "$VMDIR" ] && echo "=== systemctl stubs ===" && sudo cat "$VMDIR/rootfs/usr/bin/systemctl" 2>/dev/null | head -5 || echo "(no /usr/bin/systemctl stub)"
    [ -n "$VMDIR" ] && echo "=== /usr/sbin/systemctl stub ===" && sudo cat "$VMDIR/rootfs/usr/sbin/systemctl" 2>/dev/null | head -5 || echo "(no /usr/sbin/systemctl stub)"
    [ -n "$VMDIR" ] && echo "=== agent.json ===" && sudo cat "$VMDIR/rootfs/var/vcap/bosh/agent.json" 2>/dev/null || echo "(no agent.json)"
    [ -n "$VMDIR" ] && echo "=== warden-cpi-agent-env.json (mbus/blobstore) ===" && sudo python3 -c "import json; d=json.load(open('$VMDIR/rootfs/var/vcap/bosh/warden-cpi-agent-env.json')); print('mbus:',d.get('mbus','(none)')[:80]); bs=d.get('env',{}).get('bosh',{}).get('blobstores',[]); print('blobstores:',len(bs))" 2>/dev/null || echo "(no warden-cpi-agent-env.json)"
    [ -n "$VMDIR" ] && echo "=== port 6868 reachable? ===" && (nc -z ${DIRECTOR_IP} 6868 2>/dev/null && echo "REACHABLE" || echo "NOT REACHABLE")
    [ -n "$VMDIR" ] && echo "=== monit postgres log ===" && sudo cat "$VMDIR/rootfs/var/vcap/bosh/log/monit-postgres.log" 2>/dev/null | tail -20 || echo "(no monit-postgres.log)"
    [ -n "$VMDIR" ] && echo "=== monit director log ===" && sudo cat "$VMDIR/rootfs/var/vcap/bosh/log/monit-director.log" 2>/dev/null | tail -20 || echo "(no monit-director.log)"
    ;;

  worker)
    VMDIR=$(sudo find ${STORE_DIR}/vms -maxdepth 2 -name 'rootfs' -type d 2>/dev/null | head -1)
    if [ -n "$VMDIR" ]; then
      echo "=== monit-req.log (last 30) ===" && sudo cat "$VMDIR/var/vcap/bosh/log/monit-req.log" 2>/dev/null | tail -30 || echo "(not found)"
      echo "=== monit-worker_1.log ===" && sudo cat "$VMDIR/var/vcap/bosh/log/monit-worker_1.log" 2>/dev/null | tail -30 || echo "(not found)"
      echo "=== monit-worker_2.log ===" && sudo cat "$VMDIR/var/vcap/bosh/log/monit-worker_2.log" 2>/dev/null | tail -20 || echo "(not found)"
      echo "=== monit-director.log (tail) ===" && sudo cat "$VMDIR/var/vcap/bosh/log/monit-director.log" 2>/dev/null | tail -20 || echo "(not found)"
      echo "=== worker_1 stdout log ===" && sudo cat "$VMDIR/var/vcap/sys/log/director/worker_1.stdout.log" 2>/dev/null | tail -20 || echo "(not found)"
      echo "=== worker_1 stderr log ===" && sudo cat "$VMDIR/var/vcap/sys/log/director/worker_1.stderr.log" 2>/dev/null | tail -20 || echo "(not found)"
      echo "=== worker_ctl exists ===" && sudo ls -la "$VMDIR/var/vcap/jobs/director/bin/worker_ctl" 2>/dev/null || echo "(not found)"
      echo "=== bpm.yml processes ===" && sudo cat "$VMDIR/var/vcap/jobs/director/config/bpm.yml" 2>/dev/null | grep "name:" | head -20 || echo "(not found)"
      echo "=== /var/vcap/sys/run/director/ ===" && sudo ls -la "$VMDIR/var/vcap/sys/run/director/" 2>/dev/null || echo "(not found)"
      echo "=== LXC console log (worker events) ===" && sudo cat "$VMDIR/../../console.log" 2>/dev/null | grep -i "worker\|monit\|start_svc\|started\|failed" | tail -30 || true
    fi
    ;;

  upload)
    echo "=== upload-stemcell failed - dumping CPI task debug log ==="
    TASK_ID=$(bosh -e ${BOSH_ENV_ALIAS} tasks --client admin --client-secret admin-password --recent=1 --json 2>/dev/null | python3 -c "import sys,json; t=json.load(sys.stdin).get('Tables',[{}])[0].get('Rows',[]); print(t[0].get('id','')) if t else ''" 2>/dev/null || echo "")
    [ -n "$TASK_ID" ] && bosh -e ${BOSH_ENV_ALIAS} task "$TASK_ID" --debug --client admin --client-secret admin-password 2>/dev/null | tail -100 || true
    echo "=== SSH diagnostics from director container ==="
    ssh -i ${SSH_KEY} \
      -o StrictHostKeyChecking=no -o ConnectTimeout=15 \
      root@${DIRECTOR_IP} '
      echo "--- CPI binary ---"
      ls -lh /var/vcap/packages/libvirt_cpi/bin/cpi 2>&1 || echo NOT FOUND
      echo "--- bundled libs ---"
      ls -lh /var/vcap/packages/libvirt_cpi/lib/ 2>&1 || echo NOT FOUND
      echo "--- ldd on CPI binary ---"
      LD_LIBRARY_PATH=/var/vcap/packages/libvirt_cpi/lib ldd /var/vcap/packages/libvirt_cpi/bin/cpi 2>&1 || true
      echo "--- cpi.json (redacted) ---"
      python3 -c "import json,sys; d=json.load(open(\"/var/vcap/jobs/libvirt_cpi/config/cpi.json\")); d.get(\"PrivateKey\") and d.__setitem__(\"PrivateKey\",\"(redacted)\"); print(json.dumps(d,indent=2))" 2>/dev/null || echo NOT FOUND
      echo "--- bin/cpi script ---"
      cat /var/vcap/jobs/libvirt_cpi/bin/cpi 2>/dev/null || echo NOT FOUND
      echo "--- run CPI directly (stderr) ---"
      echo "{}" | LD_LIBRARY_PATH=/var/vcap/packages/libvirt_cpi/lib /var/vcap/packages/libvirt_cpi/bin/cpi -configPath /var/vcap/jobs/libvirt_cpi/config/cpi.json 2>&1 | tail -30 || true
      echo "--- test SSH to host 192.168.122.1 ---"
      ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 root@192.168.122.1 "echo SSH_OK; virsh -c lxc:/// list --all" 2>&1 | head -10 || true
      ' 2>&1 || echo "(SSH into director container failed - check sshd/key setup)"
    echo "=== blobstore port 25250 diagnostics (if blobstore wait failed) ==="
    DIRVM=$(sudo find ${STORE_DIR}/vms -maxdepth 1 -mindepth 1 -type d 2>/dev/null | xargs -I{} stat -c '%W {}' {} 2>/dev/null | sort -n | head -1 | cut -d' ' -f2-)
    [ -n "$DIRVM" ] && echo "=== monit-blobstore*.log ===" && sudo tail -50 "$DIRVM/rootfs/var/vcap/bosh/log/monit-blobstore_nginx.log" "$DIRVM/rootfs/var/vcap/bosh/log/monit-blobstore.log" 2>/dev/null || echo "(no monit-blobstore*.log)"
    [ -n "$DIRVM" ] && echo "=== monit-stub.log ===" && sudo tail -30 "$DIRVM/rootfs/tmp/monit-svcs.log" 2>/dev/null || echo "(no monit-svcs.log)"
    [ -n "$DIRVM" ] && echo "=== /var/vcap/sys/run/bpm/blobstore/ ===" && sudo ls -la "$DIRVM/rootfs/var/vcap/sys/run/bpm/blobstore/" 2>/dev/null || echo "(no blobstore run dir)"
    [ -n "$DIRVM" ] && echo "=== /var/vcap/packages/nginx/logs/ ===" && sudo ls -la "$DIRVM/rootfs/var/vcap/packages/nginx/logs/" 2>/dev/null || echo "(no nginx logs dir)"
    ;;

  deploy)
    echo "=== bosh deploy failed - dumping last task log ==="
    TASK_ID=$(bosh -e ${BOSH_ENV_ALIAS} tasks --client admin --client-secret admin-password --recent=1 --json 2>/dev/null | python3 -c "import sys,json; t=json.load(sys.stdin).get('Tables',[{}])[0].get('Rows',[]); print(t[0].get('id','')) if t else ''" 2>/dev/null || echo "")
    [ -n "$TASK_ID" ] && bosh -e ${BOSH_ENV_ALIAS} task "$TASK_ID" --client admin --client-secret admin-password 2>/dev/null | tail -100 || true
    VMDIR=$(sudo find ${STORE_DIR}/vms -maxdepth 1 -mindepth 1 -type d 2>/dev/null | sort | tail -2)
    for vd in $VMDIR; do
      echo "=== VM dir: $vd ==="
      sudo ls -la "$vd/" 2>/dev/null | head -10 || true
      echo "=== bosh-agent-init.log (last 100 lines) ==="
      sudo cat "$vd/rootfs/var/vcap/bosh/log/bosh-agent-init.log" 2>/dev/null | tail -100 || echo "(no bosh-agent-init.log)"
      echo "=== blobs/tmp/ (in-progress downloads) ==="
      sudo ls -lah "$vd/rootfs/var/vcap/data/tmp" 2>/dev/null | head -20 || echo "(no blobs tmp dir)"
      sudo ls -lah "$vd/rootfs/var/vcap/packages/" 2>/dev/null | head -20 || echo "(no packages dir)"
      echo "=== warden-cpi-agent-env.json (mbus field) ==="
      sudo python3 -c "import json; f='$vd/rootfs/var/vcap/bosh/warden-cpi-agent-env.json'; d=json.load(open(f)); c=d.get('env',{}).get('bosh',{}).get('mbus',{}); print('cert.ca present:',bool(c.get('cert',{}).get('ca'))); print('cert.certificate present:',bool(c.get('cert',{}).get('certificate'))); print('cert.private_key present:',bool(c.get('cert',{}).get('private_key'))); print('mbus url:',d.get('env',{}).get('bosh',{}).get('mbus',{}).get('url') or d.get('mbus','(none)')); bs=d.get('env',{}).get('bosh',{}).get('blobstores',[]); print('env.bosh.blobstores count:',len(bs)); [print('  blobstore[0].provider:',bs[0].get('provider','?'),'endpoint:',bs[0].get('options',{}).get('endpoint','(none)')) if bs else None]" 2>/dev/null || echo "(warden-cpi-agent-env.json not found or no cert)"
      echo "=== agent current log (last 20 lines) ==="
      sudo cat "$vd/rootfs/var/vcap/bosh/log/current" 2>/dev/null | tail -20 || echo "(no current log)"
    done
    DIRVM=$(sudo find ${STORE_DIR}/vms -maxdepth 1 -mindepth 1 -type d 2>/dev/null | xargs -I{} stat -c '%W {}' {} 2>/dev/null | sort -n | head -1 | cut -d' ' -f2-)
    [ -n "$DIRVM" ] && echo "=== NATS auth.json full (director VM) ===" && sudo cat "$DIRVM/rootfs/var/vcap/data/nats/auth.json" 2>/dev/null | python3 -c "import json,sys; d=json.load(sys.stdin); users=d.get('authorization',{}).get('users',[]); print(f'{len(users)} users:'); [print('  '+u.get('user','?')) for u in users[:10]]" 2>/dev/null || echo "(no auth.json or parse error)"
    [ -n "$DIRVM" ] && echo "=== NATS server log (last 50 lines) ===" && sudo tail -50 "$DIRVM/rootfs/var/vcap/data/sys/log/nats/nats.log" 2>/dev/null || echo "(no nats.log)"
    [ -n "$DIRVM" ] && echo "=== director-subject file ===" && sudo cat "$DIRVM/rootfs/var/vcap/data/nats/director-subject" 2>/dev/null || echo "(no director-subject file)"
    [ -n "$DIRVM" ] && echo "=== hm-subject file ===" && sudo cat "$DIRVM/rootfs/var/vcap/data/nats/hm-subject" 2>/dev/null || echo "(no hm-subject file)"
    [ -n "$DIRVM" ] && echo "=== NATS port 4222 reachable from CI host? ===" && (nc -z ${DIRECTOR_IP} 4222 2>/dev/null && echo "REACHABLE" || echo "NOT REACHABLE")
    [ -n "$DIRVM" ] && echo "=== monit-nats.log (last 100 lines) ===" && sudo tail -100 "$DIRVM/rootfs/var/vcap/bosh/log/monit-nats.log" 2>/dev/null || echo "(no monit-nats.log)"
    [ -n "$DIRVM" ] && echo "=== bosh-nats-sync.log (via data path, last 50 lines) ===" && sudo tail -50 "$DIRVM/rootfs/var/vcap/data/sys/log/nats/bosh-nats-sync.log" 2>/dev/null || echo "(no bosh-nats-sync.log)"
    [ -n "$DIRVM" ] && echo "=== bosh_nats_sync running? ===" && sudo cat "$DIRVM/rootfs/var/vcap/sys/run/bpm/nats/bosh_nats_sync.pid" 2>/dev/null | xargs -I{} sh -c 'kill -0 {} 2>/dev/null && echo "pid {} alive" || echo "pid {} dead/missing"' || echo "(no bosh_nats_sync pid file)"
    [ -n "$DIRVM" ] && echo "=== director_ca_cert.pem (via data/jobs symlink) ===" && sudo find "$DIRVM/rootfs/var/vcap/data/jobs/nats" -name "director_ca_cert.pem" 2>/dev/null | head -1 | xargs -I{} sudo head -5 {} 2>/dev/null || echo "(no director_ca_cert.pem found)"
    [ -n "$DIRVM" ] && echo "=== bosh_nats_sync binary (data/jobs) ===" && sudo ls -la "$DIRVM/rootfs/var/vcap/data/jobs/nats/bin/bosh_nats_sync" "$DIRVM/rootfs/var/vcap/data/jobs/nats/bin/bosh_nats_sync.real" 2>/dev/null || echo "(no bosh_nats_sync* in data/jobs)"
    [ -n "$DIRVM" ] && echo "=== bosh_nats_sync wrapper content ===" && sudo head -5 "$DIRVM/rootfs/var/vcap/data/jobs/nats/bin/bosh_nats_sync" 2>/dev/null || echo "(no bosh_nats_sync wrapper)"
    [ -n "$DIRVM" ] && echo "=== nats bpm.yml processes ===" && sudo grep -E "name:|executable:|privileged:" "$DIRVM/rootfs/var/vcap/jobs/nats/config/bpm.yml" 2>/dev/null || sudo find "$DIRVM/rootfs/var/vcap/data/jobs/nats" -name "bpm.yml" 2>/dev/null | xargs sudo grep -E "name:|executable:|privileged:" 2>/dev/null || echo "(no bpm.yml found)"
    [ -n "$DIRVM" ] && echo "=== bosh_nats_sync_config.yml ===" && sudo find "$DIRVM/rootfs/var/vcap/data/jobs/nats" -name "bosh_nats_sync_config.yml" 2>/dev/null | head -1 | xargs -I{} sudo cat {} 2>/dev/null | head -30 || echo "(no bosh_nats_sync_config.yml found)"
    [ -n "$DIRVM" ] && echo "=== /var/vcap/data/sys/log/nats/ ===" && sudo ls -la "$DIRVM/rootfs/var/vcap/data/sys/log/nats/" 2>/dev/null || echo "(no data/sys/log/nats/ dir)"
    [ -n "$DIRVM" ] && echo "=== monit-blobstore*.log (last 30 lines) ===" && sudo tail -30 "$DIRVM/rootfs/var/vcap/bosh/log/monit-blobstore_nginx.log" "$DIRVM/rootfs/var/vcap/bosh/log/monit-blobstore.log" 2>/dev/null || echo "(no monit-blobstore*.log)"
    [ -n "$DIRVM" ] && echo "=== nginx error.log (blobstore) ===" && sudo tail -30 "$DIRVM/rootfs/var/vcap/packages/nginx/logs/error.log" 2>/dev/null || echo "(no nginx error.log)"
    [ -n "$DIRVM" ] && echo "=== /var/vcap/store/blobstore/store/ (blob count) ===" && sudo find "$DIRVM/rootfs/var/vcap/store/blobstore/store" -type f 2>/dev/null | wc -l | xargs echo "blob files:" || echo "(no blobstore store dir)"
    [ -n "$DIRVM" ] && echo "=== blobstore curl test from CI host ===" && curl -sk --connect-timeout 5 --max-time 10 -o /dev/null -w "HTTP %{http_code} in %{time_total}s\n" https://${DIRECTOR_IP}:25250/ 2>&1 || echo "(curl failed)"
    [ -n "$DIRVM" ] && echo "=== /var/vcap/data/blobs/ (blob count and listing) ===" && sudo ls -la "$DIRVM/rootfs/var/vcap/data/blobs/" 2>/dev/null | head -30 || echo "(no /var/vcap/data/blobs/ dir)"
    [ -n "$DIRVM" ] && echo "=== /var/vcap/data/blobs/ dir ownership ===" && sudo stat "$DIRVM/rootfs/var/vcap/data/blobs" 2>/dev/null || echo "(no /var/vcap/data/blobs dir)"
    [ -n "$DIRVM" ] && echo "=== cpi-inject.log (wrapper install status) ===" && sudo tail -20 "$DIRVM/rootfs/var/vcap/bosh/log/cpi-inject.log" 2>/dev/null || echo "(no cpi-inject.log)"
    [ -n "$DIRVM" ] && echo "=== bosh_nats_sync pgrep (via nsenter) ===" && sudo nsenter --target $(sudo cat "$DIRVM/rootfs/var/vcap/sys/run/bpm/nats/nats.pid" 2>/dev/null || echo 1) --pid -- pgrep -a bosh_nats_sync 2>/dev/null || echo "(pgrep unavailable or none)"
    [ -n "$DIRVM" ] && echo "=== nats pre-start log ===" && sudo cat "$DIRVM/rootfs/var/vcap/data/sys/log/nats/pre-start.stderr.log" "$DIRVM/rootfs/var/vcap/data/sys/log/nats/pre-start.stdout.log" 2>/dev/null | tail -20 || echo "(no nats pre-start logs)"
    sudo virsh -c lxc:/// list --all 2>/dev/null || true
    echo "=== bosh vms (all deployments) ===" && \
      bosh -e ${BOSH_ENV_ALIAS} vms --client admin --client-secret admin-password 2>/dev/null || echo "(bosh vms failed)"
    ;;

  verify)
    echo "=== nats port 4222 on 192.168.122.20 never opened - diagnostics ==="
    NATSVM=$(sudo find ${STORE_DIR}/vms -maxdepth 1 -mindepth 1 -type d 2>/dev/null | xargs -I{} stat -c '%W {}' {} 2>/dev/null | sort -n | tail -1 | cut -d' ' -f2-)
    [ -n "$NATSVM" ] && echo "=== deployed nats VM dir: $NATSVM ===" && sudo ls -la "$NATSVM/" 2>/dev/null || true
    [ -n "$NATSVM" ] && echo "=== bosh-agent-init.log (nats VM) ===" && sudo tail -50 "$NATSVM/rootfs/var/vcap/bosh/log/bosh-agent-init.log" 2>/dev/null || echo "(no bosh-agent-init.log)"
    [ -n "$NATSVM" ] && echo "=== monit-nats.log (nats VM) ===" && sudo tail -100 "$NATSVM/rootfs/var/vcap/bosh/log/monit-nats.log" 2>/dev/null || echo "(no monit-nats.log)"
    [ -n "$NATSVM" ] && echo "=== nats-server log ===" && sudo tail -50 "$NATSVM/rootfs/var/vcap/sys/log/nats/nats.log" 2>/dev/null || echo "(no nats.log)"
    [ -n "$NATSVM" ] && echo "=== bosh_nats_sync log ===" && sudo tail -50 "$NATSVM/rootfs/var/vcap/sys/log/nats/bosh-nats-sync.log" 2>/dev/null || echo "(no bosh-nats-sync.log)"
    [ -n "$NATSVM" ] && echo "=== /var/vcap/data/nats/ ===" && sudo ls -la "$NATSVM/rootfs/var/vcap/data/nats/" 2>/dev/null || echo "(no data/nats/)"
    [ -n "$NATSVM" ] && echo "=== nats.cfg ===" && sudo cat "$NATSVM/rootfs/var/vcap/jobs/nats/config/nats.cfg" 2>/dev/null | grep -v "BEGIN CERT\|END CERT\|BEGIN RSA\|END RSA" || echo "(no nats.cfg)"
    [ -n "$NATSVM" ] && echo "=== bosh_nats_sync_config.yml ===" && sudo cat "$NATSVM/rootfs/var/vcap/jobs/nats/config/bosh_nats_sync_config.yml" 2>/dev/null | head -20 || echo "(no bosh_nats_sync_config.yml)"
    [ -n "$NATSVM" ] && echo "=== nats bpm.yml ===" && sudo cat "$NATSVM/rootfs/var/vcap/jobs/nats/config/bpm.yml" 2>/dev/null || echo "(no bpm.yml)"
    [ -n "$NATSVM" ] && echo "=== /var/vcap/sys/run/bpm/nats/ ===" && sudo ls -la "$NATSVM/rootfs/var/vcap/sys/run/bpm/nats/" 2>/dev/null || echo "(no run/bpm/nats/)"
    sudo virsh -c lxc:/// list --all 2>/dev/null || true
    ;;

  *) echo "e2e-debug-lxc.sh: unknown phase '$PHASE'";;
esac
exit 0
