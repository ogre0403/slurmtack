(function () {
  'use strict';

  var state = {
    token: localStorage.getItem('slurmtack_token') || '',
    partitions: [],
    nodes: [],
    selectedPartition: null,
    history: [],
    historyOldest: null
  };

  function authHeaders() {
    var h = { 'Content-Type': 'application/json' };
    if (state.token) h['Authorization'] = 'Bearer ' + state.token;
    return h;
  }

  function showError(msg) {
    var el = document.getElementById('error-banner');
    el.textContent = msg;
    el.style.display = msg ? 'block' : 'none';
  }

  function hideLoading() {
    document.getElementById('loading-overlay').classList.add('hidden');
  }

  // Health check
  async function checkHealth() {
    var badge = document.getElementById('health-badge');
    try {
      var res = await fetch('/api/health');
      var data = await res.json().catch(function () { return {}; });
      if (res.ok && data.status === 'ok') {
        badge.className = 'healthy';
        badge.textContent = 'healthy';
      } else {
        badge.className = 'unhealthy';
        badge.textContent = 'unhealthy';
      }
    } catch (e) {
      badge.className = 'unhealthy';
      badge.textContent = 'unavailable';
    }
  }

  // Inventory
  async function loadInventory(partition) {
    var url = '/v1/dashboard/inventory';
    if (partition) url += '?partition=' + encodeURIComponent(partition);
    try {
      var res = await fetch(url, { headers: authHeaders() });
      if (!res.ok) {
        showError('Failed to load inventory (HTTP ' + res.status + ')');
        hideLoading();
        return;
      }
      var data = await res.json();
      state.partitions = data.partitions || [];
      state.nodes = data.nodes || [];
      showError('');
      renderPartitions();
      renderNodes();
    } catch (e) {
      showError('Failed to load inventory: ' + e.message);
    }
    hideLoading();
  }

  function renderPartitions() {
    var list = document.getElementById('partition-list');
    list.innerHTML = '';
    var allLi = document.createElement('li');
    allLi.textContent = 'All';
    allLi.className = state.selectedPartition === null ? 'active' : '';
    allLi.onclick = function () { state.selectedPartition = null; loadInventory(); };
    list.appendChild(allLi);

    state.partitions.forEach(function (p) {
      var li = document.createElement('li');
      li.textContent = p.name + ' (' + (p.nodes ? p.nodes.length : 0) + ')';
      li.className = state.selectedPartition === p.name ? 'active' : '';
      li.onclick = function () { state.selectedPartition = p.name; loadInventory(p.name); };
      list.appendChild(li);
    });
  }

  function renderNodes() {
    var grid = document.getElementById('node-grid');
    grid.innerHTML = '';
    var nodes = state.nodes;
    if (state.selectedPartition) {
      nodes = nodes.filter(function (n) {
        return n.partitions && n.partitions.indexOf(state.selectedPartition) >= 0;
      });
    }

    nodes.forEach(function (node) {
      var card = document.createElement('div');
      card.className = 'node-card';

      var ownerClass = 'owner-' + (node.owner || 'unknown');
      var slurmState = node.slurm ? node.slurm.state : '-';
      var osEnabled = node.openstack ? (node.openstack.compute_service.enabled ? 'enabled' : 'disabled') : '-';

      var activeInfo = '';
      if (node.switch && node.switch.active_execution_id) {
        activeInfo = '<div class="meta">Active: ' + escapeHtml(node.switch.active_state) + '</div>';
      }

      var lastInfo = '';
      if (node.last_execution) {
        lastInfo = '<div class="meta">Last: ' + escapeHtml(node.last_execution.overall_status) + ' (' + escapeHtml(node.last_execution.direction) + ')</div>';
      }

      card.innerHTML =
        '<h3>' + escapeHtml(node.node_name) + ' <span class="owner-badge ' + ownerClass + '">' + escapeHtml(node.owner) + '</span></h3>' +
        '<div class="meta">Slurm: ' + escapeHtml(slurmState) + ' | OS: ' + escapeHtml(osEnabled) + '</div>' +
        activeInfo + lastInfo +
        '<div class="actions">' + buildNodeActions(node) + '</div>';

      grid.appendChild(card);
    });

    // Populate history node filter
    var nodeFilter = document.getElementById('history-node-filter');
    var currentVal = nodeFilter.value;
    nodeFilter.innerHTML = '<option value="">All nodes</option>';
    state.nodes.forEach(function (n) {
      var opt = document.createElement('option');
      opt.value = n.node_name;
      opt.textContent = n.node_name;
      nodeFilter.appendChild(opt);
    });
    nodeFilter.value = currentVal;
  }

  function buildNodeActions(node) {
    if (node.switch && node.switch.active_execution_id) {
      return '<button class="danger" onclick="cancelExecution(\'' + escapeAttr(node.switch.active_execution_id) + '\')">Cancel</button>';
    }
    if (node.available_direction === 'openstack_to_slurm') {
      return '<button onclick="switchNode(\'' + escapeAttr(node.node_name) + '\', \'openstack_to_slurm\')">Switch to Slurm</button>';
    }
    if (node.available_direction === 'slurm_to_openstack') {
      return '<button onclick="switchFromPartition(\'' + escapeAttr(node.node_name) + '\')">Switch to OpenStack</button>';
    }
    return '';
  }

  // History
  async function loadHistory(append) {
    var nodeFilter = document.getElementById('history-node-filter').value;
    var statusFilter = document.getElementById('history-status-filter').value;
    var url = '/v1/switches?limit=20';
    if (nodeFilter) url += '&node=' + encodeURIComponent(nodeFilter);
    if (statusFilter) url += '&status=' + encodeURIComponent(statusFilter);
    if (append && state.historyOldest) url += '&before=' + encodeURIComponent(state.historyOldest);

    try {
      var res = await fetch(url, { headers: authHeaders() });
      if (!res.ok) return;
      var data = await res.json();
      if (!append) state.history = [];
      state.history = state.history.concat(data);
      if (data.length > 0) {
        state.historyOldest = data[data.length - 1].requested_at;
      }
      renderHistory();
      document.getElementById('history-load-more').style.display = data.length >= 20 ? 'block' : 'none';
    } catch (e) { /* silent */ }
  }

  function renderHistory() {
    var list = document.getElementById('history-list');
    list.innerHTML = '';
    state.history.forEach(function (exec) {
      var li = document.createElement('li');
      li.innerHTML =
        '<span class="history-status ' + escapeAttr(exec.overall_status) + '"></span>' +
        '<strong>' + escapeHtml(exec.node_name || '(pending)') + '</strong> ' +
        escapeHtml(exec.direction) + '<br>' +
        '<small>' + escapeHtml(exec.current_state) + ' - ' + formatTime(exec.requested_at) + '</small>';
      li.onclick = function () { openDetail(exec.id); };
      list.appendChild(li);
    });
  }

  // Detail drawer
  async function openDetail(id) {
    var drawer = document.getElementById('detail-drawer');
    var title = document.getElementById('detail-title');
    var content = document.getElementById('detail-content');
    drawer.classList.add('open');
    title.textContent = 'Execution ' + id;
    content.innerHTML = 'Loading...';

    try {
      var [execRes, stepsRes] = await Promise.all([
        fetch('/v1/switches/' + encodeURIComponent(id), { headers: authHeaders() }),
        fetch('/v1/switches/' + encodeURIComponent(id) + '/steps', { headers: authHeaders() })
      ]);
      var exec = await execRes.json();
      var steps = await stepsRes.json();

      var html = '<div class="meta">';
      html += '<p><strong>Direction:</strong> ' + escapeHtml(exec.direction) + '</p>';
      html += '<p><strong>Node:</strong> ' + escapeHtml(exec.node_name || '(pending)') + '</p>';
      html += '<p><strong>State:</strong> ' + escapeHtml(exec.current_state) + '</p>';
      html += '<p><strong>Status:</strong> ' + escapeHtml(exec.overall_status) + '</p>';
      html += '<p><strong>Requested:</strong> ' + formatTime(exec.requested_at) + '</p>';
      html += '<p><strong>By:</strong> ' + escapeHtml(exec.requested_by) + '</p>';
      if (exec.error_summary) html += '<p><strong>Error:</strong> ' + escapeHtml(exec.error_summary) + '</p>';
      html += '</div>';

      html += '<h3 style="margin-top:16px;font-size:0.9rem;">Steps</h3>';
      html += '<ul class="step-timeline">';
      if (Array.isArray(steps)) {
        steps.forEach(function (s) {
          html += '<li>';
          html += '<span class="step-status ' + escapeAttr(s.status) + '">' + escapeHtml(s.status) + '</span> ';
          html += escapeHtml(s.step_name);
          if (s.host) html += ' <small>(' + escapeHtml(s.host) + ')</small>';
          if (s.exit_code !== null && s.exit_code !== undefined) html += ' <small>exit=' + s.exit_code + '</small>';
          html += '</li>';
        });
      }
      html += '</ul>';
      content.innerHTML = html;
    } catch (e) {
      content.innerHTML = 'Failed to load details.';
    }
  }

  window.closeDetail = function () {
    document.getElementById('detail-drawer').classList.remove('open');
  };

  // Switch actions
  window.switchNode = async function (nodeName, direction) {
    if (!confirm('Switch ' + nodeName + ' (' + direction + ')?')) return;
    var requestedBy = prompt('Requested by:', 'dashboard-operator');
    if (!requestedBy) return;

    try {
      var body = { direction: direction, node_name: nodeName, requested_by: requestedBy };
      var res = await fetch('/v1/switches', { method: 'POST', headers: authHeaders(), body: JSON.stringify(body) });
      if (!res.ok) {
        var err = await res.json().catch(function () { return {}; });
        alert('Switch failed: ' + (err.error || res.status));
        return;
      }
      loadInventory(state.selectedPartition);
      loadHistory(false);
    } catch (e) {
      alert('Switch failed: ' + e.message);
    }
  };

  window.switchFromPartition = async function (nodeName) {
    if (!confirm('Switch ' + nodeName + ' to OpenStack from partition ' + (state.selectedPartition || '(auto)') + '?')) return;
    var requestedBy = prompt('Requested by:', 'dashboard-operator');
    if (!requestedBy) return;

    try {
      var body = { direction: 'slurm_to_openstack', requested_by: requestedBy };
      if (state.selectedPartition) body.slurm_partition = state.selectedPartition;
      var res = await fetch('/v1/switches', { method: 'POST', headers: authHeaders(), body: JSON.stringify(body) });
      if (!res.ok) {
        var err = await res.json().catch(function () { return {}; });
        alert('Switch failed: ' + (err.error || res.status));
        return;
      }
      loadInventory(state.selectedPartition);
      loadHistory(false);
    } catch (e) {
      alert('Switch failed: ' + e.message);
    }
  };

  window.cancelExecution = async function (id) {
    if (!confirm('Cancel execution ' + id + '?')) return;
    try {
      var res = await fetch('/v1/switches/' + encodeURIComponent(id) + '/cancel', { method: 'POST', headers: authHeaders() });
      if (!res.ok) {
        var err = await res.json().catch(function () { return {}; });
        alert('Cancel failed: ' + (err.error || res.status));
        return;
      }
      loadInventory(state.selectedPartition);
      loadHistory(false);
    } catch (e) {
      alert('Cancel failed: ' + e.message);
    }
  };

  // Utilities
  function escapeHtml(s) {
    if (!s) return '';
    return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  function escapeAttr(s) {
    return escapeHtml(s).replace(/'/g, '&#39;');
  }

  function formatTime(iso) {
    if (!iso) return '-';
    try {
      var d = new Date(iso);
      return d.toLocaleString();
    } catch (e) { return iso; }
  }

  // Token prompt
  function ensureToken() {
    if (!state.token) {
      state.token = prompt('Enter API token:', '') || '';
      if (state.token) localStorage.setItem('slurmtack_token', state.token);
    }
  }

  // History filter events
  document.getElementById('history-node-filter').onchange = function () { state.historyOldest = null; loadHistory(false); };
  document.getElementById('history-status-filter').onchange = function () { state.historyOldest = null; loadHistory(false); };
  document.getElementById('history-load-more').onclick = function () { loadHistory(true); };

  // Init
  ensureToken();
  checkHealth();
  loadInventory(null);
  loadHistory(false);

  // Periodic refresh
  setInterval(checkHealth, 30000);
  setInterval(function () { loadInventory(state.selectedPartition); }, 30000);
  setInterval(function () { state.historyOldest = null; loadHistory(false); }, 30000);
})();
