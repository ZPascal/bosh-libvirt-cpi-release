#!/usr/bin/env bash
# QEMU E2E failure diagnostics. Invoked as: e2e-debug-qemu.sh <phase>
# phase: wait | worker | upload | deploy | verify
# Reads: DIRECTOR_IP, STORE_DIR, SSH_KEY, BOSH_ENV_ALIAS (exported by e2e.yml)
set +e  # diagnostics must never abort; every probe is best-effort
PHASE="${1:-unknown}"
: "${STORE_DIR:=/tmp/bosh_libvirt_cpi}"
: "${SSH_KEY:=/tmp/bosh-libvirt-ssh-key}"

case "$PHASE" in
  wait)
    echo "=== network diagnostics ==="
    ip route show || true
    sudo arp -n | grep "192.168.122" || true
    ping -c3 -W2 ${DIRECTOR_IP} 2>&1 || true
    curl -svk --max-time 10 https://${DIRECTOR_IP}:25555/info 2>&1 | tail -20 || true
    echo "=== bosh create-env log (last 80 lines) ===" && tail -80 /tmp/bosh-create-env.log || true
    echo "=== QEMU console log (monit/postgres/director events - first 40) ===" && \
      sudo find /tmp -name 'bosh-vm-*-console.log' 2>/dev/null | xargs sudo cat 2>/dev/null | grep -i "\[monit\]\|postgres\|director\|nats\|Error\|5432\|25555\|start_svc\|pre-start\|bpm\.yml\|never ready\|started.*pid\|4222\|ss port\|ps nats" | head -40 || true
    echo "=== QEMU console log (monit/postgres/director events - last 40) ===" && \
      sudo find /tmp -name 'bosh-vm-*-console.log' 2>/dev/null | xargs sudo cat 2>/dev/null | grep -i "\[monit\]\|postgres\|director\|nats\|Error\|5432\|25555\|start_svc\|pre-start\|bpm\.yml\|never ready\|started.*pid\|4222\|ss port\|ps nats" | tail -40 || true
    echo "=== QEMU console log (last 30 lines) ===" && \
      sudo find /tmp -name 'bosh-vm-*-console.log' 2>/dev/null | xargs sudo cat 2>/dev/null | tail -30 || true
    echo "=== Destroying QEMU VM so rootfs.img can be mounted ===" && \
      QEMU_DOM=$(sudo virsh -c qemu:///system list --name 2>/dev/null | grep -v '^$' | head -1) && \
      if [ -n "$QEMU_DOM" ]; then sudo virsh -c qemu:///system destroy "$QEMU_DOM" 2>/dev/null && echo "Destroyed $QEMU_DOM" || echo "Destroy failed (ok)"; else echo "No running domains found"; fi
    echo "=== Mounting QEMU rootfs.img to read service logs ===" && {
      QEMU_IMG=$(sudo find ${STORE_DIR}/vms -name 'rootfs.img' 2>/dev/null | head -1)
      if [ -n "$QEMU_IMG" ]; then
        sudo mkdir -p /mnt/qemu-rootfs
        sudo mount -o loop,ro,noload "$QEMU_IMG" /mnt/qemu-rootfs 2>/dev/null && {
          echo "=== rootfs root listing ===" && sudo ls -la /mnt/qemu-rootfs/ 2>/dev/null | head -30 || echo "(mount succeeded but ls failed)"
          echo "=== /bosh-init content (first 5 lines) ===" && sudo head -5 /mnt/qemu-rootfs/bosh-init 2>/dev/null || echo "(no /bosh-init)"
          echo "=== monit-console-fallback.log ===" && sudo cat /mnt/qemu-rootfs/tmp/monit-console.log 2>/dev/null | tail -30 || echo "(not found)"
          echo "=== monit-stub.log ===" && sudo cat /mnt/qemu-rootfs/tmp/monit-stub.log 2>/dev/null | tail -30 || echo "(not found)"
          echo "=== monit-req.log ===" && sudo cat /mnt/qemu-rootfs/var/vcap/bosh/log/monit-req.log 2>/dev/null | tail -50 || echo "(not found)"
          echo "=== monit-nats.log (head) ===" && sudo cat /mnt/qemu-rootfs/var/vcap/bosh/log/monit-nats.log 2>/dev/null | head -30 || echo "(not found)"
          echo "=== monit-nats.log (tail) ===" && sudo cat /mnt/qemu-rootfs/var/vcap/bosh/log/monit-nats.log 2>/dev/null | tail -30 || echo "(not found)"
          echo "=== bosh-nats-sync.log ===" && sudo cat /mnt/qemu-rootfs/var/vcap/data/sys/log/nats/bosh-nats-sync.log 2>/dev/null | tail -30 || echo "(not found)"
          echo "=== /var/vcap/data/sys/log/nats/ ===" && sudo ls -la /mnt/qemu-rootfs/var/vcap/data/sys/log/nats/ 2>/dev/null || echo "(not found)"
          echo "=== bosh_nats_sync binary (data/jobs) ===" && sudo ls -la /mnt/qemu-rootfs/var/vcap/data/jobs/nats/bin/bosh_nats_sync /mnt/qemu-rootfs/var/vcap/data/jobs/nats/bin/bosh_nats_sync.real 2>/dev/null || echo "(no bosh_nats_sync* in data/jobs)"
          echo "=== bosh_nats_sync wrapper content ===" && sudo head -5 /mnt/qemu-rootfs/var/vcap/data/jobs/nats/bin/bosh_nats_sync 2>/dev/null || echo "(no wrapper)"
          echo "=== NATS auth.json ===" && sudo cat /mnt/qemu-rootfs/var/vcap/data/nats/auth.json 2>/dev/null | python3 -c "import json,sys; d=json.load(sys.stdin); users=d.get('authorization',{}).get('users',[]); print(f'{len(users)} users')" 2>/dev/null || echo "(no auth.json)"
          echo "=== monit-postgres.log (head) ===" && sudo cat /mnt/qemu-rootfs/var/vcap/bosh/log/monit-postgres.log 2>/dev/null | head -30 || echo "(not found)"
          echo "=== monit-postgres.log (tail) ===" && sudo cat /mnt/qemu-rootfs/var/vcap/bosh/log/monit-postgres.log 2>/dev/null | tail -30 || echo "(not found)"
          echo "=== postgres bpm.yml ===" && sudo find /mnt/qemu-rootfs/var/vcap/data/jobs -name bpm.yml -path '*/postgres/*' 2>/dev/null | head -1 | xargs sudo cat 2>/dev/null | head -40 || echo "(not found)"
          echo "=== data jobs listing ===" && sudo ls /mnt/qemu-rootfs/var/vcap/data/jobs/ 2>/dev/null | head -20 || echo "(not found)"
          echo "=== store dir listing ===" && sudo ls -la /mnt/qemu-rootfs/var/vcap/store/ 2>/dev/null || echo "(not found)"
          echo "=== postgres-15 data dir listing ===" && sudo ls -la /mnt/qemu-rootfs/var/vcap/store/postgres-15/ 2>/dev/null | head -20 || echo "(not found)"
          echo "=== monit-director.log ===" && sudo cat /mnt/qemu-rootfs/var/vcap/bosh/log/monit-director.log 2>/dev/null | tail -30 || echo "(not found)"
          echo "=== monit-svcs.log ===" && sudo cat /mnt/qemu-rootfs/tmp/monit-svcs.log 2>/dev/null | tail -10 || echo "(not found)"
          echo "=== bosh-agent-init.log ===" && sudo cat /mnt/qemu-rootfs/var/vcap/bosh/log/bosh-agent-init.log 2>/dev/null | tail -20 || echo "(not found)"
          echo "=== bosh-agent/current log (tail) ===" && sudo cat /mnt/qemu-rootfs/var/vcap/bosh/log/current 2>/dev/null | grep -v "unlimitedRetry\|DelayedAudit" | tail -40 || echo "(not found)"
          echo "=== nats job config dir ===" && sudo ls /mnt/qemu-rootfs/var/vcap/jobs/nats/config/ 2>/dev/null || echo "(not found)"
          echo "=== nats bpm.yml ===" && sudo cat /mnt/qemu-rootfs/var/vcap/jobs/nats/config/bpm.yml 2>/dev/null | head -20 || echo "(not found)"
          echo "=== director nginx config check ===" && sudo ls /mnt/qemu-rootfs/var/vcap/jobs/director/config/ 2>/dev/null || echo "(not found)"
          sudo umount /mnt/qemu-rootfs 2>/dev/null
        } || echo "Mount failed"
      else
        echo "No rootfs.img found"
      fi
    } || true
    ;;

  worker)
    echo "=== QEMU console log (worker/monit events) ===" && \
      sudo find /tmp -name 'bosh-vm-*-console.log' 2>/dev/null | xargs sudo cat 2>/dev/null | grep -i "worker\|monit\|start_svc\|started\|failed\|error" | tail -40 || true
    QEMU_IMG=$(sudo find ${STORE_DIR}/vms -name 'rootfs.img' 2>/dev/null | head -1)
    if [ -n "$QEMU_IMG" ]; then
      QEMU_DOM=$(sudo virsh -c qemu:///system list --name 2>/dev/null | grep -v '^$' | head -1)
      [ -n "$QEMU_DOM" ] && sudo virsh -c qemu:///system destroy "$QEMU_DOM" 2>/dev/null || true
      sudo mkdir -p /mnt/qemu-rootfs-w
      sudo mount -o loop,ro,noload "$QEMU_IMG" /mnt/qemu-rootfs-w 2>/dev/null && {
        echo "=== monit-req.log (last 30) ===" && sudo cat /mnt/qemu-rootfs-w/var/vcap/bosh/log/monit-req.log 2>/dev/null | tail -30 || echo "(not found)"
        echo "=== monit-worker_1.log ===" && sudo cat /mnt/qemu-rootfs-w/var/vcap/bosh/log/monit-worker_1.log 2>/dev/null | tail -30 || echo "(not found)"
        echo "=== monit-worker_2.log ===" && sudo cat /mnt/qemu-rootfs-w/var/vcap/bosh/log/monit-worker_2.log 2>/dev/null | tail -20 || echo "(not found)"
        echo "=== monit-director.log (tail) ===" && sudo cat /mnt/qemu-rootfs-w/var/vcap/bosh/log/monit-director.log 2>/dev/null | tail -20 || echo "(not found)"
        echo "=== worker_1 stdout log ===" && sudo cat /mnt/qemu-rootfs-w/var/vcap/sys/log/director/worker_1.stdout.log 2>/dev/null | tail -20 || echo "(not found)"
        echo "=== worker_1 stderr log ===" && sudo cat /mnt/qemu-rootfs-w/var/vcap/sys/log/director/worker_1.stderr.log 2>/dev/null | tail -20 || echo "(not found)"
        echo "=== worker_ctl script exists ===" && sudo ls -la /mnt/qemu-rootfs-w/var/vcap/jobs/director/bin/worker_ctl 2>/dev/null || echo "(not found)"
        echo "=== bpm.yml processes ===" && sudo cat /mnt/qemu-rootfs-w/var/vcap/jobs/director/config/bpm.yml 2>/dev/null | grep "name:" | head -20 || echo "(not found)"
        echo "=== /var/vcap/sys/run/director/ ===" && sudo ls -la /mnt/qemu-rootfs-w/var/vcap/sys/run/director/ 2>/dev/null || echo "(not found)"
        sudo umount /mnt/qemu-rootfs-w 2>/dev/null || true
      } || echo "Mount failed"
    fi
    ;;

  upload)
    echo "=== upload-stemcell failed - dumping CPI task debug log ==="
    TASK_ID=$(bosh -e ${BOSH_ENV_ALIAS} tasks --client admin --client-secret admin-password --recent=1 --json 2>/dev/null | python3 -c "import sys,json; t=json.load(sys.stdin).get('Tables',[{}])[0].get('Rows',[]); print(t[0].get('id','')) if t else ''" 2>/dev/null || echo "")
    [ -n "$TASK_ID" ] && bosh -e ${BOSH_ENV_ALIAS} task "$TASK_ID" --debug --client admin --client-secret admin-password 2>/dev/null | tail -100 || true
    echo "=== SSH diagnostics from director VM ==="
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
      ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 root@192.168.122.1 "echo SSH_OK; virsh -c qemu:///system list --all" 2>&1 | head -10 || true
      ' 2>&1 || echo "(SSH into director VM failed - check sshd/key setup)"
    echo "=== QEMU console logs (per-VM, last 50 lines each) ===" && \
      sudo find /tmp -name 'bosh-vm-*-console.log' 2>/dev/null | while read f; do echo "--- $f ---"; sudo tail -50 "$f" 2>/dev/null || true; done || true
    echo "=== blobstore port 25250 diagnostics (if blobstore wait failed) ==="
    QEMU_IMG=$(sudo find ${STORE_DIR}/vms -name 'rootfs.img' 2>/dev/null | head -1)
    if [ -n "$QEMU_IMG" ]; then
      sudo mkdir -p /mnt/qemu-rootfs-blob
      sudo mount -o loop,ro,noload "$QEMU_IMG" /mnt/qemu-rootfs-blob 2>/dev/null && {
        echo "=== monit-blobstore*.log ===" && sudo tail -50 /mnt/qemu-rootfs-blob/var/vcap/bosh/log/monit-blobstore_nginx.log /mnt/qemu-rootfs-blob/var/vcap/bosh/log/monit-blobstore.log 2>/dev/null || echo "(no monit-blobstore*.log)"
        echo "=== /var/vcap/sys/run/bpm/blobstore/ ===" && sudo ls -la /mnt/qemu-rootfs-blob/var/vcap/sys/run/bpm/blobstore/ 2>/dev/null || echo "(no blobstore run dir)"
        echo "=== /var/vcap/packages/nginx/logs/ ===" && sudo ls -la /mnt/qemu-rootfs-blob/var/vcap/packages/nginx/logs/ 2>/dev/null || echo "(no nginx logs dir)"
        sudo umount /mnt/qemu-rootfs-blob 2>/dev/null
      } || echo "Mount failed"
    fi
    ;;

  deploy)
    echo "=== bosh deploy failed - dumping last task log ==="
    TASK_ID=$(bosh -e ${BOSH_ENV_ALIAS} tasks --client admin --client-secret admin-password --recent=1 --json 2>/dev/null | python3 -c "import sys,json; t=json.load(sys.stdin).get('Tables',[{}])[0].get('Rows',[]); print(t[0].get('id','')) if t else ''" 2>/dev/null || echo "")
    [ -n "$TASK_ID" ] && bosh -e ${BOSH_ENV_ALIAS} task "$TASK_ID" --client admin --client-secret admin-password 2>/dev/null | tail -100 || true
    echo "=== bosh deploy debug log (CPI calls with needs_recovery / nr_check output) ==="
    [ -n "$TASK_ID" ] && bosh -e ${BOSH_ENV_ALIAS} task "$TASK_ID" --debug --client admin --client-secret admin-password 2>/dev/null | grep -E "nr_check|nr_verify|nr_raw_fix|tune2fs|e2fsck|needs_recovery|python3|verify.py|bit cleared|already clear" | tail -50 || true
    echo "=== QEMU console logs (per-VM, last 50 lines each) ===" && \
      sudo find /tmp -name 'bosh-vm-*-console.log' 2>/dev/null | while read f; do echo "--- $f ---"; sudo tail -50 "$f" 2>/dev/null || true; done || true
    echo "=== stemcell ext4 info ===" && {
      SC_IMG=$(sudo find ${STORE_DIR}/stemcells -name 'image.ext4' 2>/dev/null | head -1)
      if [ -n "$SC_IMG" ]; then
        echo "Stemcell image: $SC_IMG ($(sudo du -sh "$SC_IMG" 2>/dev/null | cut -f1))"
        sudo file "$SC_IMG" 2>/dev/null || true
        sudo tune2fs -l "$SC_IMG" 2>/dev/null | head -10 || echo "(tune2fs failed)"
      else
        echo "No stemcell image.ext4 found"
      fi
    } || true
    echo "=== compilation VM rootfs.img info ===" && {
      for ROOTFS in $(sudo find ${STORE_DIR}/vms -name 'rootfs.img' 2>/dev/null); do
        echo "VM image: $ROOTFS ($(sudo du -sh "$ROOTFS" 2>/dev/null | cut -f1))"
        sudo file "$ROOTFS" 2>/dev/null || true
        sudo tune2fs -l "$ROOTFS" 2>/dev/null | head -10 || echo "(tune2fs failed)"
        echo "--- needs_recovery superblock bit (offset 1120, bit 0x04) ---"
        sudo python3 -c "import struct; f=open('$ROOTFS','rb'); f.seek(1120); v=struct.unpack('<I',f.read(4))[0]; f.close(); print('s_feature_incompat=0x%08x needs_recovery_bit=%d' % (v, (v>>2)&1))" 2>/dev/null || echo "(python3 bit check failed)"
        sudo mkdir -p /mnt/compile-rootfs
        sudo mount -o loop,ro,noload "$ROOTFS" /mnt/compile-rootfs 2>/dev/null && {
          echo "--- rootfs root listing ---"
          sudo ls -la /mnt/compile-rootfs/ 2>/dev/null | head -20
          echo "--- bosh-init script (last 15 lines) ---"
          sudo tail -15 /mnt/compile-rootfs/bosh-init 2>/dev/null || echo "(no bosh-init)"
          echo "--- agent init log ---"
          sudo tail -20 /mnt/compile-rootfs/var/vcap/bosh/log/bosh-agent-init.log 2>/dev/null || echo "(no bosh-agent-init.log)"
          echo "--- agent current log ---"
          sudo tail -20 /mnt/compile-rootfs/var/vcap/bosh/log/current 2>/dev/null || echo "(no current log)"
          echo "--- warden-cpi-agent-env.json blobstore config ---"
          sudo python3 -c "import json; d=json.load(open('/mnt/compile-rootfs/var/vcap/bosh/warden-cpi-agent-env.json')); bs=d.get('env',{}).get('bosh',{}).get('blobstores',[]); print('env.bosh.blobstores count:',len(bs)); [print('  provider:',b.get('provider'),'endpoint:',b.get('options',{}).get('endpoint','(none)')) for b in bs[:2]]; tb=d.get('blobstore',{}); print('top-level blobstore provider:',tb.get('provider','(none)'),); mbus=d.get('mbus','(none)'); print('mbus:',mbus[:60])" 2>/dev/null || echo "(warden-cpi-agent-env.json not found)"
          echo "--- agent.json (infra type) ---"
          sudo python3 -c "import json; d=json.load(open('/mnt/compile-rootfs/var/vcap/bosh/agent.json')); print(json.dumps(d,indent=2)[:400])" 2>/dev/null || echo "(agent.json not found)"
          echo "--- bosh-agent-restart.log ---"
          sudo tail -10 /mnt/compile-rootfs/var/vcap/bosh/log/bosh-agent-restart.log 2>/dev/null || echo "(no bosh-agent-restart.log)"
          echo "--- dhclient log ---"
          sudo cat /mnt/compile-rootfs/tmp/dhclient.log 2>/dev/null | tail -10 || echo "(no dhclient.log)"
          sudo umount /mnt/compile-rootfs 2>/dev/null
        } || echo "(mount of $ROOTFS failed)"
      done
    } || true
    echo "=== QEMU blobstore log (via SSH) ===" && \
      ssh -i ${SSH_KEY} -o StrictHostKeyChecking=no -o ConnectTimeout=10 \
        root@${DIRECTOR_IP} 'tail -30 /var/vcap/bosh/log/monit-blobstore_nginx.log /var/vcap/bosh/log/monit-blobstore.log 2>/dev/null || echo "(no monit-blobstore*.log)"' 2>/dev/null || echo "(SSH unavailable)"
    echo "=== CPI log (post-cleanup diagnostics) ===" && \
      ssh -i ${SSH_KEY} -o StrictHostKeyChecking=no -o ConnectTimeout=10 \
        root@${DIRECTOR_IP} 'grep -h "post-cleanup\|e2fsck\|tune2fs\|needs_recovery\|nr_check\|nr_verify\|nr_raw_fix\|verify.py" /var/vcap/bosh/log/libvirt_cpi/*.log /var/vcap/data/jobs/libvirt_cpi/*/*/libvirt_cpi.stdout.log 2>/dev/null | tail -50 || echo "(no CPI log entries)"' 2>/dev/null || echo "(SSH unavailable)"
    echo "=== clear.py / verify.py scripts on host (leftover if python3 failed mid-run) ===" && \
      sudo find ${STORE_DIR} -name '*.clear.py' -o -name '*.verify.py' 2>/dev/null | head -5 | while read f; do echo "--- $f ---"; sudo cat "$f" | head -5; done || echo "(no leftover scripts found)"
    echo "=== NATS port 4222 reachable from CI host? ===" && \
      (nc -z ${DIRECTOR_IP} 4222 2>/dev/null && echo "REACHABLE" || echo "NOT REACHABLE (connection refused - NATS not listening on 0.0.0.0?)")
    echo "=== NATS listen address (via SSH to director) ===" && \
      ssh -i ${SSH_KEY} -o StrictHostKeyChecking=no -o ConnectTimeout=10 \
        root@${DIRECTOR_IP} 'ss -tlnp | grep 4222 || echo "(nothing on 4222)"' 2>/dev/null || echo "(SSH unavailable)"
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
    ;;

  *) echo "e2e-debug-qemu.sh: unknown phase '$PHASE'";;
esac
exit 0
