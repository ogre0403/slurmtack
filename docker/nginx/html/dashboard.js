(function () {
  'use strict';

  var PAGE_SIZE = 10;

  var state = {
    token: localStorage.getItem('slurmtack_token') || '',
    partitions: [],
    nodes: [],
    selectedPartition: null,
    executions: [],
    execPage: 0,
    execPageCursors: [null],
    execHasMore: false,
    selectedExecutionId: null
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

  function renderPartitionActionBar() {
    var bar = document.getElementById('partition-action-bar');
    var hasSlurmNodes = state.nodes.some(function (n) {
      return n.available_direction === 'slurm_to_openstack';
    });
    if (!hasSlurmNodes) {
      bar.style.display = 'none';
      return;
    }
    var label = state.selectedPartition
      ? 'Switch from partition ' + escapeHtml(state.selectedPartition) + ' to OpenStack'
      : 'Switch from All partitions to OpenStack';
    bar.style.display = 'flex';
    bar.innerHTML =
      '<span class="partition-label">' + label + '</span>' +
      '<button onclick="switchFromPartition()">Switch to OpenStack</button>';
  }

  function renderNodes() {
    var grid = document.getElementById('node-grid');
    grid.innerHTML = '';
    var nodes = state.nodes.slice().sort(function (a, b) {
      return a.node_name < b.node_name ? -1 : a.node_name > b.node_name ? 1 : 0;
    });
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

    // Populate execution node filter
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

    renderPartitionActionBar();
  }

  function buildNodeActions(node) {
    if (node.switch && node.switch.active_execution_id) {
      return '<button class="danger" onclick="cancelExecution(\'' + escapeAttr(node.switch.active_execution_id) + '\')">Cancel</button>';
    }
    if (node.available_direction === 'openstack_to_slurm') {
      return '<button onclick="switchNode(\'' + escapeAttr(node.node_name) + '\', \'openstack_to_slurm\')">Switch to Slurm</button>';
    }
    return '';
  }

  // Executions
  async function loadExecutions(pageIndex) {
    if (pageIndex === undefined) pageIndex = 0;
    var nodeFilter = document.getElementById('history-node-filter').value;
    var statusFilter = document.getElementById('history-status-filter').value;
    var url = '/v1/switches?limit=' + PAGE_SIZE;
    if (nodeFilter) url += '&node=' + encodeURIComponent(nodeFilter);
    if (statusFilter) url += '&status=' + encodeURIComponent(statusFilter);
    var cursor = state.execPageCursors[pageIndex];
    if (cursor) url += '&before=' + encodeURIComponent(cursor);

    try {
      var res = await fetch(url, { headers: authHeaders() });
      if (!res.ok) return;
      var data = await res.json();
      state.executions = data;
      state.execPage = pageIndex;
      state.execHasMore = data.length >= PAGE_SIZE;
      if (state.execHasMore && !state.execPageCursors[pageIndex + 1]) {
        state.execPageCursors[pageIndex + 1] = data[data.length - 1].requested_at;
      }
      renderExecutions();
    } catch (e) { /* silent */ }
  }

  function renderExecutions() {
    var list = document.getElementById('execution-list');
    list.innerHTML = '';
    state.executions.forEach(function (exec) {
      var li = document.createElement('li');
      var isActive = exec.overall_status === 'active';
      if (isActive) li.classList.add('active-exec');
      if (state.selectedExecutionId === exec.id) li.classList.add('selected');
      var cancelBtn = isActive
        ? '<div class="exec-row-actions"><button class="exec-cancel danger" onclick="event.stopPropagation();cancelExecution(\'' + escapeAttr(exec.id) + '\')">Cancel</button></div>'
        : '';
      li.innerHTML =
        '<span class="history-status ' + escapeAttr(exec.overall_status) + '"></span>' +
        '<strong>' + escapeHtml(exec.id) + '</strong><br>' +
        '<small>' + escapeHtml(exec.direction) + ' &mdash; ' + escapeHtml(exec.current_state) + ' &mdash; ' + formatTime(exec.requested_at) + '</small>' +
        cancelBtn;
      li.onclick = function () { selectExecution(exec.id); };
      list.appendChild(li);
    });

    document.getElementById('exec-page-prev').disabled = state.execPage === 0;
    document.getElementById('exec-page-next').disabled = !state.execHasMore;
    document.getElementById('exec-page-info').textContent = 'Page ' + (state.execPage + 1);
  }

  function selectExecution(id) {
    state.selectedExecutionId = id;
    renderExecutions();
    openDetail(id);
  }

  window.execNextPage = function () {
    if (state.execHasMore) loadExecutions(state.execPage + 1);
  };

  window.execPrevPage = function () {
    if (state.execPage > 0) loadExecutions(state.execPage - 1);
  };

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
      html += '<p class="exec-current-state"><strong>Current State:</strong> ' + escapeHtml(exec.current_state) + '</p>';
      html += '<p><strong>Status:</strong> ' + escapeHtml(exec.overall_status) + '</p>';
      html += '<p><strong>Direction:</strong> ' + escapeHtml(exec.direction) + '</p>';
      html += '<p><strong>Node:</strong> ' + escapeHtml(exec.node_name || '(pending)') + '</p>';
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
      state.execPage = 0;
      state.execPageCursors = [null];
      loadExecutions(0);
    } catch (e) {
      alert('Switch failed: ' + e.message);
    }
  };

  window.switchFromPartition = async function () {
    var partitionLabel = state.selectedPartition || 'All';
    if (!confirm('Start slurm_to_openstack switch for partition: ' + partitionLabel + '?')) return;
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
      state.execPage = 0;
      state.execPageCursors = [null];
      loadExecutions(0);
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
      state.execPage = 0;
      state.execPageCursors = [null];
      await loadExecutions(0);
      if (state.selectedExecutionId) openDetail(state.selectedExecutionId);
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

  // Filter events - reset pagination when filters change
  document.getElementById('history-node-filter').onchange = function () {
    state.execPage = 0;
    state.execPageCursors = [null];
    loadExecutions(0);
  };
  document.getElementById('history-status-filter').onchange = function () {
    state.execPage = 0;
    state.execPageCursors = [null];
    loadExecutions(0);
  };

  // Init
  ensureToken();
  checkHealth();
  loadInventory(null);
  loadExecutions(0);

  // Periodic refresh
  setInterval(checkHealth, 30000);
  setInterval(function () { loadInventory(state.selectedPartition); }, 30000);
  setInterval(function () { loadExecutions(state.execPage); }, 30000);
})();
