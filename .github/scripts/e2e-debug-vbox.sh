#!/usr/bin/env bash
# VBox E2E failure diagnostics. Invoked as: e2e-debug-vbox.sh <phase>
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
    sudo arp -n | grep "192.168.1" || true
    ping -c3 -W2 ${DIRECTOR_IP} 2>&1 || true
    curl -svk --max-time 10 https://${DIRECTOR_IP}:25555/info 2>&1 | tail -20 || true
    echo "=== bosh create-env log (last 80 lines) ===" && tail -80 /tmp/bosh-create-env.log || true
    echo "=== VBox domain list ===" && virsh -c vbox:///session list --all 2>/dev/null || true
    echo "=== env.iso present? ===" && find ${STORE_DIR}/vms -name 'env.iso' 2>/dev/null | head -5 || true
    echo "=== env.iso size ===" && find ${STORE_DIR}/vms -name 'env.iso' 2>/dev/null | head -1 | xargs ls -lh 2>/dev/null || true
    echo "=== VM store dirs ===" && ls -la ${STORE_DIR}/vms/ 2>/dev/null | head -20 || true
    ;;

  worker)
    echo "=== VBox domain list ===" && virsh -c vbox:///session list --all 2>/dev/null || true
    echo "=== bosh create-env log (last 50 lines) ===" && tail -50 /tmp/bosh-create-env.log || true
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
      echo "--- cpi.json (redacted) ---"
      python3 -c "import json,sys; d=json.load(open(\"/var/vcap/jobs/libvirt_cpi/config/cpi.json\")); d.get(\"PrivateKey\") and d.__setitem__(\"PrivateKey\",\"(redacted)\"); print(json.dumps(d,indent=2))" 2>/dev/null || echo NOT FOUND
      echo "--- run CPI directly (stderr) ---"
      echo "{}" | LD_LIBRARY_PATH=/var/vcap/packages/libvirt_cpi/lib /var/vcap/packages/libvirt_cpi/bin/cpi -configPath /var/vcap/jobs/libvirt_cpi/config/cpi.json 2>&1 | tail -30 || true
      echo "--- test virsh vbox connection ---"
      virsh -c vbox:///session list --all 2>&1 | head -10 || true
      ' 2>&1 || echo "(SSH into director VM failed)"
    echo "=== VBox domain list ===" && virsh -c vbox:///session list --all 2>/dev/null || true
    ;;

  deploy)
    echo "=== bosh deploy failed - dumping last task log ==="
    TASK_ID=$(bosh -e ${BOSH_ENV_ALIAS} tasks --client admin --client-secret admin-password --recent=1 --json 2>/dev/null | python3 -c "import sys,json; t=json.load(sys.stdin).get('Tables',[{}])[0].get('Rows',[]); print(t[0].get('id','')) if t else ''" 2>/dev/null || echo "")
    [ -n "$TASK_ID" ] && bosh -e ${BOSH_ENV_ALIAS} task "$TASK_ID" --client admin --client-secret admin-password 2>/dev/null | tail -100 || true
    echo "=== VBox domain list ===" && virsh -c vbox:///session list --all 2>/dev/null || true
    echo "=== VM store dirs ===" && ls -la ${STORE_DIR}/vms/ 2>/dev/null | head -20 || true
    echo "=== env.iso files ===" && find ${STORE_DIR}/vms -name 'env.iso' 2>/dev/null | xargs ls -lh 2>/dev/null || true
    echo "=== bosh vms (all deployments) ===" && \
      bosh -e ${BOSH_ENV_ALIAS} vms --client admin --client-secret admin-password 2>/dev/null || echo "(bosh vms failed)"
    ;;

  verify)
    echo "=== bosh deploy verify failed - diagnostics ==="
    echo "=== VBox domain list ===" && virsh -c vbox:///session list --all 2>/dev/null || true
    echo "=== NATS port 4222 reachable? ===" && \
      (nc -z ${DIRECTOR_IP} 4222 2>/dev/null && echo "REACHABLE" || echo "NOT REACHABLE")
    ;;

  *) echo "e2e-debug-vbox.sh: unknown phase '$PHASE'";;
esac
exit 0
