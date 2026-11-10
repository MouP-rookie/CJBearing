const { createApp, markRaw } = Vue;

createApp({
  data() {
    return {
      view: 'env',
      loading: false,
      error: '',
      exportNotice: '',
      generatedAt: '',
      // ---- 环境监测 ----
      limit: 10,
      rangeMode: '0',
      start: '',
      end: '',
      dataMode: '',
      dataStart: '',
      dataEnd: '',
      refreshSeconds: 10,
      countdown: 10,
      power: null,
      pressure: null,
      th: null,
      now: Date.now(),
      detail: { visible: false, title: '', rows: [] },
      // ---- 设备历史数据 ----
      devices: [],
      devicesLoading: false,
      devicesLoaded: false,
      typeFilter: '',
      selectedId: '',
      deviceQuery: '',
      deviceOpen: false,
      deviceHighlight: 0,
      historyRange: '7',
      historyStart: '',
      historyEnd: '',
      historyProgress: '',
      historyAbort: null,
      historyBounds: null,
      history: null,
      historyRequestId: 0,
      historyNotice: '',
      latestReportedAt: 0,
      selectedGroups: [],
      // 质量数据 / 设备数据 分开查看
      dataView: 'quality',
      fillDown: true,
      showAllItems: false,
      onlyOut: false,
      // 分页: 1000 条 × 65 字段会有 6.5 万个单元格, 必须只渲染当前页
      pageSize: 100,
      pageIndex: 0,
      jumpPageNo: 1,
      spcPage: 0,
      spcJumpPageNo: 1,
      rawModal: {
        visible: false,
        loading: false,
        title: '',
        meta: '',
        fullText: '',
        preview: '',
        sizeText: '',
        previewSize: 300 * 1024,
        showFull: false,
        copied: false
      }
    };
  },
  computed: {
    powerRows() { return this.power ? this.power.rows : []; },
    pressureRows() { return this.pressure ? this.pressure.rows : []; },
    thRows() { return this.th ? this.th.rows : []; },
    powerSpan() {
      const rows = this.powerRows;
      if (!rows.length) { return '无数据'; }
      const oldest = rows[rows.length - 1];
      const newest = rows[0];
      const tail = newest.running ? '至今' : (newest.endTime || '—');
      return oldest.startTime + ' ~ ' + tail;
    },
    pressureSpan() { return this.spanOf(this.pressureRows); },
    thSpan() { return this.spanOf(this.thRows); },
    deviceTypes() {
      const set = new Set();
      this.devices.forEach(d => { if (d.type) { set.add(d.type); } });
      return Array.from(set).sort();
    },
    /** 模糊搜索结果: 先按名称、再按标签, 支持空格分词, 命中的越精确越靠前 */
    deviceMatches() {
      const list = this.devices.filter(d => !this.typeFilter || d.type === this.typeFilter);
      const query = (this.deviceQuery || '').trim().toLowerCase();
      if (!query) {
        return this.sortDevices(list).slice(0, 200);
      }
      const tokens = query.split(/\s+/).filter(Boolean);
      const hits = [];
      for (const device of list) {
        const score = this.matchScore(device, tokens);
        if (score >= 0) {
          hits.push({ device, score });
        }
      }
      hits.sort((a, b) => a.score - b.score || this.compareDevice(a.device, b.device));
      return hits.slice(0, 80).map(hit => hit.device);
    },
    /** 质量数据 = 检测项(测量值/上限/下限) + spcvalue 项 */
    qualityGroups() {
      if (!this.history) { return []; }
      return (this.history.groups || []).filter(g => g.source !== 'plain');
    },
    /** 设备数据 = 其余运转类字段(code/status/rhythm/workNum/goodNum 等) */
    deviceGroups() {
      if (!this.history) { return []; }
      return (this.history.groups || []).filter(g => g.source === 'plain');
    },
    currentGroups() {
      return this.dataView === 'device' ? this.deviceGroups : this.qualityGroups;
    },
    activeGroups() {
      const keys = this.selectedGroups;
      return this.currentGroups.filter(g => keys.includes(g.valueKey));
    },
    // Count abnormal observations across the complete range, using selected columns only.
    // The same millisecond timestamp counts once even when several columns are OUT.
    abnormalTimestamps() {
      const timestamps = new Set();
      if (!this.history || !this.activeGroups.length) { return timestamps; }
      const keys = this.activeGroups.map(group => group.valueKey);
      for (const row of this.history.rows || []) {
        const ts = Number(row.ts);
        if (Number.isFinite(ts) && keys.some(key => row.flags && row.flags[key] === 'OUT')) {
          timestamps.add(ts);
        }
      }
      return timestamps;
    },
    selectedOutCount() {
      return this.abnormalTimestamps.size;
    },
    /** 当前页签可选的字段: 质量页默认只列可判定项, 设备页列全部运转字段 */
    chipGroups() {
      if (this.dataView === 'device') {
        return this.deviceGroups;
      }
      return this.showAllItems
        ? this.qualityGroups
        : this.qualityGroups.filter(g => this.isJudgeable(g));
    },
    emptyHint() {
      if (this.dataView === 'device') {
        return '该设备没有上报运转类字段（状态 / 节拍 / 计数）';
      }
      return '该设备没有上报质量检测项（测量值 / 上限 / 下限）';
    },
    hasLimitSelected() {
      return this.activeGroups.some(g => this.isJudgeable(g));
    },
    columnCount() {
      return 1 + this.activeGroups.length;
    },
    pageCount() {
      return Math.max(1, Math.ceil(this.visibleRows.length / this.pageSize));
    },
    safePageIndex() {
      return Math.min(Math.max(this.pageIndex, 0), this.pageCount - 1);
    },
    pagedRows() {
      const start = this.safePageIndex * this.pageSize;
      return this.visibleRows.slice(start, start + this.pageSize);
    },
    spcRows() {
      return this.history && this.history.spcRows ? this.history.spcRows : [];
    },
    spcOutCount() {
      const selected = new Map(this.activeGroups.map(group => [group.name, !!group.booleanCheck]));
      const timestamps = new Set();
      for (const row of this.spcRows) {
        if (!selected.has(row.name)) { continue; }
        const booleanOut = selected.get(row.name) && row.value !== null && row.value !== '' && Number(row.value) === 0;
        const ts = Number(row.ts);
        if ((row.out || booleanOut) && Number.isFinite(ts)) { timestamps.add(ts); }
      }
      return timestamps.size;
    },
    spcPageCount() {
      return Math.max(1, Math.ceil(this.spcRows.length / this.pageSize));
    },
    safeSpcPage() {
      return Math.min(Math.max(this.spcPage, 0), this.spcPageCount - 1);
    },
    pagedSpcRows() {
      const start = this.safeSpcPage * this.pageSize;
      return this.spcRows.slice(start, start + this.pageSize);
    },
    /**
     * 设备数据页可按"空值沿用上一次上报值"补全。
     * 设备分批上报, 同一个时间点只有部分字段有值; 补一次之后每行都是各字段的最近状态, 更完整也更好读。
     * 质量页不补: 那批数据本来就是同一时间戳上报的, 补值只会把空行带进表格。
     */
    filledRows() {
      const h = this.history;
      if (!h) { return []; }
      const rows = h.rows || [];
      if (this.dataView !== 'device' || !this.fillDown) {
        return rows;
      }
      const allKeys = new Set(this.deviceGroups.map(group => group.valueKey));
      const last = {};
      const out = new Array(rows.length);
      for (let i = rows.length - 1; i >= 0; i--) {
        const row = rows[i];
        const values = {};
        allKeys.forEach(key => { if (row.values[key] !== undefined) { values[key] = row.values[key]; } });
        allKeys.forEach((key) => {
          const value = values[key];
          if (value === undefined || value === '') {
            if (last[key] !== undefined) {
              values[key] = last[key];
            }
          } else {
            last[key] = value;
          }
        });
        out[i] = { ts: row.ts, time: row.time, values, flags: row.flags };
      }
      return out;
    },
    visibleRows() {
      const h = this.history;
      if (!h) { return []; }
      let rows = this.filledRows;
      const groups = this.activeGroups;
      if (!groups.length) { return []; }
      rows = rows.filter(r => groups.some(g => r.values[g.valueKey] !== undefined
        && r.values[g.valueKey] !== ''));
      if (this.dataView === 'quality' && this.onlyOut) {
        const timestamps = this.abnormalTimestamps;
        rows = rows.filter(r => timestamps.has(Number(r.ts)));
      }
      return rows;
    }
  },
  methods: {
    // ===================== 环境监测 =====================
    onRangeChange() {
      if (this.rangeMode === 'custom') {
        return;
      }
      this.start = '';
      this.end = '';
      this.refresh(false);
    },
    resetRange() {
      this.rangeMode = '0';
      this.start = '';
      this.end = '';
      this.refresh(false);
    },
    queryPower() {
      if (this.rangeMode === 'custom' && !this.start && !this.end) {
        this.error = '请先选择开始时间和结束时间，再点查询';
        return;
      }
      this.refresh(false);
    },
    queryData() {
      if (this.dataMode === 'custom' && !this.dataStart && !this.dataEnd) {
        this.error = '请先选择开始时间和结束时间，再点查询';
        return;
      }
      this.refresh(false);
    },
    onDataRangeChange() {
      if (this.dataMode === 'custom') {
        return;
      }
      this.dataStart = '';
      this.dataEnd = '';
      this.refresh(false);
    },
    resetDataRange() {
      this.dataMode = '';
      this.dataStart = '';
      this.dataEnd = '';
      this.refresh(false);
    },
    spanOf(rows) {
      if (!rows.length) { return '无数据'; }
      return rows[rows.length - 1].collectTime + ' ~ ' + rows[0].collectTime;
    },
    queryUrl(force) {
      const parts = ['limit=' + this.limit, 'refresh=' + (force ? 'true' : 'false')];
      if (this.rangeMode === 'custom' && (this.start || this.end)) {
        if (this.start) { parts.push('start=' + encodeURIComponent(this.start)); }
        if (this.end) { parts.push('end=' + encodeURIComponent(this.end)); }
      } else {
        parts.push('days=' + (this.rangeMode === 'custom' ? 0 : Number(this.rangeMode)));
      }
      if (this.dataMode === 'custom' && (this.dataStart || this.dataEnd)) {
        if (this.dataStart) { parts.push('dataStart=' + encodeURIComponent(this.dataStart)); }
        if (this.dataEnd) { parts.push('dataEnd=' + encodeURIComponent(this.dataEnd)); }
      } else if (this.dataMode && this.dataMode !== 'custom') {
        parts.push('dataDays=' + Number(this.dataMode));
      }
      return '/api/overview?' + parts.join('&');
    },
    display(value) {
      return value === null || value === undefined || value === '' ? '—' : value;
    },
    async refresh(force) {
      this.loading = true;
      try {
        const data = await this.getJson(this.queryUrl(force));
        this.power = data.power;
        this.pressure = data.pressure;
        this.th = data.th;
        this.generatedAt = data.generatedAt;
        this.error = '';
        this.countdown = this.refreshSeconds;
      } catch (e) {
        this.error = '数据获取失败：' + (e.message || String(e));
      } finally {
        this.loading = false;
      }
    },
    durationOf(row) {
      if (!row.running) {
        return row.durationText;
      }
      const seconds = Math.max(0, Math.floor((this.now - row.startTs) / 1000));
      return this.formatDuration(seconds);
    },
    formatDuration(seconds) {
      const days = Math.floor(seconds / 86400);
      const hours = Math.floor((seconds % 86400) / 3600);
      const minutes = Math.floor((seconds % 3600) / 60);
      const secs = seconds % 60;
      return days + '天' + hours + '小时' + minutes + '分' + secs + '秒';
    },
    openPressureDetail(row) {
      this.detail = {
        visible: true,
        title: row.deviceName + ' 采集详情 · ' + row.collectTime,
        rows: row.raw || []
      };
    },
    openThDetail(row) {
      this.detail = {
        visible: true,
        title: row.deviceName + ' 采集详情 · ' + row.collectTime,
        rows: row.raw || []
      };
    },
    closeDetail() {
      this.detail = { visible: false, title: '', rows: [] };
    },

    // ===================== 设备历史数据 =====================
    async getJson(url, signal) {
      const resp = await fetch(url, { cache: 'no-store', signal });
      const text = await resp.text();
      let data = null;
      try {
        data = JSON.parse(text);
      } catch (e) {
        throw new Error('返回内容不是 JSON：' + text.slice(0, 120));
      }
      if (!resp.ok) {
        throw new Error(data && data.error ? data.error : ('HTTP ' + resp.status));
      }
      return data;
    },
    switchView(view) {
      this.view = view;
      this.error = '';
      if (view === 'device' && !this.devicesLoaded) {
        this.loadDevices(false);
      }
    },
    async loadDevices(force) {
      this.devicesLoading = true;
      try {
        const data = await this.getJson('/api/devices' + (force ? '?refresh=true' : ''));
        this.devices = data.devices || [];
        this.devicesLoaded = true;
        this.error = '';
      } catch (e) {
        this.error = '设备目录加载失败：' + (e.message || String(e));
      } finally {
        this.devicesLoading = false;
      }
    },
    deviceText(d) {
      const name = d.name || d.id;
      return d.label ? name + ' · ' + d.label : name;
    },
    compareDevice(a, b) {
      const ca = (a.name || '').startsWith('CZDG') ? 0 : 1;
      const cb = (b.name || '').startsWith('CZDG') ? 0 : 1;
      if (ca !== cb) {
        return ca - cb;
      }
      return (a.name || '').localeCompare(b.name || '');
    },
    sortDevices(list) {
      return list.slice().sort((a, b) => this.compareDevice(a, b));
    },
    /** 单个关键词的匹配得分, 越小越精确; -1 表示不匹配 */
    tokenScore(device, token) {
      const name = (device.name || '').toLowerCase();
      const label = (device.label || '').toLowerCase();
      if (name.startsWith(token)) { return 0; }
      if (name.includes(token)) { return 1; }
      if (label.startsWith(token)) { return 2; }
      if (label.includes(token)) { return 3; }
      if (this.isSubsequence(name, token)) { return 4; }
      if (this.isSubsequence(label, token)) { return 5; }
      return -1;
    },
    /** 子序列匹配: 允许中间跳过字符, 用于"模糊"命中 */
    isSubsequence(text, token) {
      let i = 0;
      for (let k = 0; k < text.length && i < token.length; k++) {
        if (text[k] === token[i]) {
          i += 1;
        }
      }
      return i === token.length;
    },
    matchScore(device, tokens) {
      let total = 0;
      for (const token of tokens) {
        const score = this.tokenScore(device, token);
        if (score < 0) {
          return -1;
        }
        total += score;
      }
      return total;
    },
    openDeviceList() {
      this.deviceOpen = true;
      this.deviceHighlight = 0;
    },
    onDeviceInput() {
      this.selectedId = '';
      this.invalidateHistory();
      this.deviceOpen = true;
      this.deviceHighlight = 0;
    },
    moveDeviceHighlight(delta) {
      const total = this.deviceMatches.length;
      if (!total) {
        return;
      }
      this.deviceOpen = true;
      this.deviceHighlight = (this.deviceHighlight + delta + total) % total;
    },
    onDeviceEnter() {
      const device = this.deviceMatches[this.deviceHighlight];
      if (device) {
        this.pickDevice(device);
      }
    },
    pickDevice(device) {
      this.invalidateHistory();
      this.selectedId = device.id;
      this.deviceQuery = this.deviceText(device);
      this.deviceOpen = false;
    },
    closeDeviceList() {
      this.deviceOpen = false;
    },
    invalidateHistory() {
      ++this.historyRequestId;
      if (this.historyAbort) { this.historyAbort.abort(); this.historyAbort = null; }
      this.historyProgress = '';
      this.historyBounds = null;
      this.history = null;
      this.historyNotice = '';
      this.latestReportedAt = 0;
      this.loading = false;
    },
    clearDevice() {
      this.invalidateHistory();
      this.selectedId = '';
      this.deviceQuery = '';
      this.deviceOpen = false;
      this.history = null;
    },
    prevPage() {
      if (this.safePageIndex > 0) {
        this.pageIndex = this.safePageIndex - 1;
      }
    },
    nextPage() {
      if (this.safePageIndex < this.pageCount - 1) {
        this.pageIndex = this.safePageIndex + 1;
      }
    },
    /** 跳转到指定页(超出范围自动夹到首/尾页) */
    gotoPage() {
      const page = Math.round(Number(this.jumpPageNo));
      if (!Number.isFinite(page)) {
        this.jumpPageNo = this.safePageIndex + 1;
        return;
      }
      this.pageIndex = Math.min(Math.max(page, 1), this.pageCount) - 1;
      this.jumpPageNo = this.safePageIndex + 1;
    },
    gotoSpcPage() {
      const page = Math.round(Number(this.spcJumpPageNo));
      if (!Number.isFinite(page)) {
        this.spcJumpPageNo = this.safeSpcPage + 1;
        return;
      }
      this.spcPage = Math.min(Math.max(page, 1), this.spcPageCount) - 1;
      this.spcJumpPageNo = this.safeSpcPage + 1;
    },
    resetPaging() {
      this.pageIndex = 0;
      this.spcPage = 0;
    },
    /** 切换 质量数据 / 设备数据 页签 */
    switchDataView(view) {
      if (this.dataView === view) {
        return;
      }
      this.dataView = view;
      this.onlyOut = false;
      this.selectedGroups = this.defaultSelection(view);
      this.resetPaging();
    },
    /** 某类数据的默认勾选: 质量页默认勾可判定项, 设备页默认全勾 */
    defaultSelection(view) {
      const groups = view === 'device' ? this.deviceGroups : this.qualityGroups;
      if (view === 'device') {
        return groups.map(g => g.valueKey);
      }
      const judgeable = groups.filter(g => this.isJudgeable(g)).map(g => g.valueKey);
      return judgeable.length ? judgeable : groups.slice(0, 12).map(g => g.valueKey);
    },
    /** 原样展示当前设备/时间范围/条数对应的原始上报报文 */
    async openRaw() {
      if (!this.selectedId) {
        this.error = '请先选择设备';
        return;
      }
      this.rawModal.loading = true;
      this.rawModal.visible = true;
      this.rawModal.copied = false;
      this.rawModal.showFull = false;
      this.rawModal.fullText = '';
      this.rawModal.preview = '';
      try {
        const data = await this.getJson(this.historyUrl('/api/history/raw'));
        const text = JSON.stringify(data, null, 1);
        const previewSize = this.rawModal.previewSize;
        const keyCount = data && typeof data === 'object' ? Object.keys(data).length : 0;
        const kb = Math.max(1, Math.round(text.length / 1024));
        this.rawModal.fullText = text;
        this.rawModal.preview = text.length > previewSize ? text.slice(0, previewSize) : text;
        this.rawModal.sizeText = kb + ' KB';
        const device = this.history ? (this.history.deviceName || this.history.deviceId) : this.selectedId;
        this.rawModal.title = device + ' · 原始报文';
        this.rawModal.meta = '字段 ' + keyCount + ' 个 · ' + kb + ' KB'
          + (this.history ? ' · ' + this.history.startTime + ' ~ ' + this.history.endTime : '');
        this.error = '';
      } catch (e) {
        this.error = '原始报文读取失败：' + (e.message || String(e));
        this.rawModal.visible = false;
      } finally {
        this.rawModal.loading = false;
      }
    },
    closeRaw() {
      this.rawModal.visible = false;
    },
    async copyRaw() {
      try {
        await navigator.clipboard.writeText(this.rawModal.fullText);
        this.rawModal.copied = true;
        setTimeout(() => { this.rawModal.copied = false; }, 2000);
      } catch (e) {
        this.error = '复制失败，可手动选中文本复制：' + (e.message || String(e));
      }
    },
    /** ===================== 导出(走后端接口, 浏览器按普通下载处理) ===================== */
    /** 导出当前表格的全部行(不受分页影响), 列与当前页签勾选的字段一致 */
    exportCsv() {
      if (!this.history) {
        this.error = '请先查询数据';
        return;
      }
      if (!this.visibleRows.length) {
        this.error = '当前查询没有可导出的数据';
        return;
      }
      const keys = this.activeGroups.map(g => g.valueKey);
      this.triggerDownload(this.exportUrl('group', keys), this.visibleRows.length);
      this.error = '';
    },
    /** 导出 spcvalue 明细(Name / Ucl / Lcl / Value) */
    exportSpcCsv() {
      if (!this.spcRows.length) {
        this.error = '当前设备没有 spcvalue 明细可导出';
        return;
      }
      this.triggerDownload(this.exportUrl('spc'), this.spcRows.length);
      this.error = '';
    },
    downloadRaw() {
      this.triggerDownload(this.exportUrl('raw'), 0);
      this.error = '';
    },
    toLocalInput(date) {
      const p = n => String(n).padStart(2, '0');
      return date.getFullYear() + '-' + p(date.getMonth() + 1) + '-' + p(date.getDate())
        + 'T' + p(date.getHours()) + ':' + p(date.getMinutes());
    },
    historyUrl(path) {
      const parts = [
        'deviceId=' + encodeURIComponent(this.selectedId),
        'order=DESC',
        ...(this.historyBounds || this.rangeParams())
      ];
      return (path || '/api/history') + '?' + parts.join('&');
    },
    rangeParams() {
      const parts = [];
      if (this.historyRange === 'custom') {
        if (this.historyStart) { parts.push('start=' + encodeURIComponent(this.historyStart)); }
        if (this.historyEnd) { parts.push('end=' + encodeURIComponent(this.historyEnd)); }
      } else if (this.historyRange === 'all') {
        parts.push('start=1970-01-02T00%3A00');
        parts.push('end=' + encodeURIComponent(this.toLocalInput(new Date())));
      } else {
        const to = new Date();
        const from = new Date(to.getTime() - Number(this.historyRange) * 86400000);
        parts.push('start=' + encodeURIComponent(this.toLocalInput(from)));
        parts.push('end=' + encodeURIComponent(this.toLocalInput(to)));
      }
      return parts;
    },
    /** 后端导出接口地址; 由浏览器直接下载, 不经过前端拼文件 */
    exportUrl(mode, keys) {
      const parts = [
        'deviceId=' + encodeURIComponent(this.selectedId),
        'order=DESC',
        'mode=' + mode,
        'fill=' + (this.dataView === 'device' && this.fillDown ? '1' : '0'),
        ...(this.historyBounds || this.rangeParams())
      ];
      if (keys && keys.length) {
        parts.push('keys=' + encodeURIComponent(keys.join(',')));
      }
      return '/api/history/export?' + parts.join('&');
    },
    /** 用隐藏 iframe 触发下载: 响应带 Content-Disposition, 浏览器不会离开当前页面 */
    triggerDownload(url, count) {
      const frame = document.createElement('iframe');
      frame.style.display = 'none';
      frame.src = url;
      document.body.appendChild(frame);
      setTimeout(() => { if (frame.parentNode) { frame.parentNode.removeChild(frame); } }, 120000);
      this.exportNotice = '已开始导出' + (count ? '（' + count + ' 条）' : '')
        + '，请查看浏览器的下载目录';
      clearTimeout(this.exportNoticeTimer);
      this.exportNoticeTimer = setTimeout(() => { this.exportNotice = ''; }, 8000);
    },
    // 兼容旧版接口：SPC 明细需要同时参与主历史表的字段和时间行。
    prepareHistory(data) {
      data.groups = data.groups || [];
      data.rows = data.rows || [];
      const rows = new Map(data.rows.map(row => [Number(row.ts), row]));
      const groups = new Map(data.groups.map(group => [group.valueKey, group]));
      for (const point of (data.spcRows || [])) {
        if (!point.name || !Number.isFinite(Number(point.ts))) { continue; }
        const key = 'spc::' + point.name;
        let group = groups.get(key) || data.groups.find(item => item.source !== 'plain' && item.name === point.name);
        if (!group) {
          const normalized = point.name.toUpperCase().replace(/ /g, '');
          group = { name: point.name, valueKey: key, upperKey: 'spcU::' + point.name, lowerKey: 'spcL::' + point.name,
            source: 'spcvalue', hasLimit: true, limitApplicable: false, judgeable: false,
            booleanCheck: /OK(?:或|\/)?NG/.test(normalized) };
          data.groups.push(group);
          groups.set(key, group);
        }
        group.limitApplicable = group.limitApplicable || point.applicable;
        group.judgeable = group.judgeable || point.applicable || group.booleanCheck;
        let row = rows.get(Number(point.ts));
        if (!row) { row = { ts: Number(point.ts), time: point.time, values: {}, flags: {} }; rows.set(row.ts, row); }
        row.values = row.values || {};
        row.flags = row.flags || {};
        if (row.values[group.valueKey] === undefined || row.values[group.valueKey] === '') {
          row.values[group.valueKey] = point.value;
          if (group.upperKey) { row.values[group.upperKey] = point.ucl; }
          if (group.lowerKey) { row.values[group.lowerKey] = point.lcl; }
          if (point.applicable) { row.flags[group.valueKey] = point.out ? 'OUT' : 'OK'; }
          else if (group.booleanCheck && (point.value === '0' || point.value === '1')) { row.flags[group.valueKey] = point.value === '0' ? 'OUT' : 'OK'; }
        }
      }
      data.rows = Array.from(rows.values()).sort((a, b) => b.ts - a.ts);
      data.dataType = data.groups.some(group => group.source !== 'plain') ? '质量数据' : '设备数据';
      data.judgeable = data.groups.some(group => this.isJudgeable(group));
      return data;
    },
    async loadLatestReport(deviceId, requestId) {
      const url = '/api/history/raw?deviceId=' + encodeURIComponent(deviceId)
        + '&start=1970-01-02T00%3A00&end=' + encodeURIComponent(this.toLocalInput(new Date())) + '&limit=1&order=DESC';
      try {
        const raw = await this.getJson(url);
        if (requestId !== this.historyRequestId || deviceId !== this.selectedId) { return; }
        let latest = 0;
        Object.values(raw).forEach(points => {
          if (Array.isArray(points)) { points.forEach(point => { const ts = Number(point.ts); if (Number.isFinite(ts)) { latest = Math.max(latest, ts); } }); }
        });
        this.latestReportedAt = latest;
        this.historyNotice = latest
          ? '当前时间范围内没有上报记录。设备最近一次上报：' + new Date(latest).toLocaleString('zh-CN', { hour12: false, timeZone: 'Asia/Shanghai' })
          : '当前时间范围内没有记录，数据源也未返回更早的上报记录。请检查设备是否已接入并上报。';
      } catch (error) {
        if (requestId === this.historyRequestId) { this.historyNotice = '当前时间范围内没有记录；最近上报时间读取失败：' + (error.message || String(error)); }
      }
    },
    queryLatestHistory() {
      if (!this.latestReportedAt) { return; }
      this.historyRange = 'custom';
      this.historyStart = this.toLocalInput(new Date(this.latestReportedAt - 86400000));
      this.historyEnd = this.toLocalInput(new Date(this.latestReportedAt + 60000));
      this.queryHistory();
    },
    async queryHistory() {
      if (!this.selectedId) { this.error = '请先选择设备'; return; }
      if (this.historyRange === 'custom' && !this.historyStart && !this.historyEnd) { this.error = '请选择自定义时间范围'; return; }
      if (this.historyRange === 'custom' && this.historyStart && this.historyEnd && this.historyStart > this.historyEnd) { this.error = '开始时间不能晚于结束时间'; return; }
      if (this.historyAbort) { this.historyAbort.abort(); }
      const abort = markRaw(new AbortController());
      this.historyAbort = abort;
      const requestId = ++this.historyRequestId;
      const deviceId = this.selectedId;
      this.history = null;
      this.historyBounds = null;
      this.historyProgress = '正在读取所选时间范围内的全部记录…';
      this.loading = true;
      this.historyNotice = '';
      this.latestReportedAt = 0;
      try {
        // Freeze both endpoints across all cursor requests, including open custom ranges.
        const end = this.historyRange === 'custom' && this.historyEnd ? this.historyEnd : this.toLocalInput(new Date());
        const start = this.historyRange === 'custom' && this.historyStart ? this.historyStart
          : this.toLocalInput(new Date(new Date(end).getTime() - Number(this.historyRange === 'custom' ? 7 : this.historyRange) * 86400000));
        const bounds = ['start=' + encodeURIComponent(start), 'end=' + encodeURIComponent(end)];
        const base = '/api/history/page?deviceId=' + encodeURIComponent(deviceId) + '&' + bounds.join('&');
        let data = null;
        let before = null;
        const groupMap = new Map();
        do {
          const page = await this.getJson(base + (before === null ? '' : '&before=' + before), abort.signal);
          if (requestId !== this.historyRequestId || deviceId !== this.selectedId) { return; }
          this.prepareHistory(page);
          if (!data) { data = { ...page, groups: [], rows: [], spcRows: [], keys: [], outCount: 0 }; }
          for (const group of page.groups) {
            const previous = groupMap.get(group.valueKey);
            if (!previous) { groupMap.set(group.valueKey, group); }
            else { for (const key of ['judgeable', 'limitApplicable', 'hasLimit']) { previous[key] = previous[key] || group[key]; } }
          }
          for (const row of page.rows) { data.rows.push(row); }
          for (const row of page.spcRows || []) { data.spcRows.push(row); }
          data.keys = Array.from(new Set([...data.keys, ...page.keys]));
          data.outCount += page.outCount || 0;
          data.judgeable = data.judgeable || page.judgeable;
          if (page.dataType === '质量数据') { data.dataType = page.dataType; }
          const next = page.nextEnd;
          if (next !== undefined && next !== null && before !== null && next >= before) { throw new Error('历史分页游标未前进，请重新查询'); }
          before = next === undefined || next === null ? null : next;
          this.historyProgress = '已读取 ' + data.rows.length.toLocaleString() + ' 个采集时间点' + (before === null ? '，正在整理分页…' : '，继续读取更早记录…');
          // Yield between batches; only the current display page is rendered after completion.
          await new Promise(resolve => setTimeout(resolve, 0));
        } while (before !== null);
        if (requestId !== this.historyRequestId || deviceId !== this.selectedId) { return; }
        data.groups = Array.from(groupMap.values());
        data.startTime = start.replace('T', ' ');
        this.historyBounds = bounds;
        this.history = markRaw(data);
        this.historyProgress = '已完整读取所选时间范围，按页显示';
        const hasQualityRows = this.qualityGroups.some(group => this.history.rows.some(row =>
          row.values[group.valueKey] !== undefined && row.values[group.valueKey] !== '' && row.values[group.valueKey] !== null));
        this.dataView = hasQualityRows || (!this.deviceGroups.length && this.qualityGroups.length) ? 'quality' : 'device';
        this.showAllItems = this.dataView === 'quality' && !this.qualityGroups.some(group => this.isJudgeable(group));
        this.selectedGroups = this.defaultSelection(this.dataView);
        this.onlyOut = false;
        this.resetPaging();
        this.error = '';
        if (!this.history.rows.length) {
          this.historyNotice = '当前时间范围内没有上报记录，正在读取最近上报时间…';
          void this.loadLatestReport(deviceId, requestId);
        }
      } catch (e) {
        if (requestId === this.historyRequestId) { this.history = null; this.historyProgress = ''; this.error = '历史数据查询失败：' + (e.message || String(e)); }
      } finally {
        if (requestId === this.historyRequestId) { this.loading = false; this.historyAbort = null; }
      }
    },
    cancelHistory() {
      this.invalidateHistory();
      this.historyProgress = '查询已取消，可调整范围后重新查询';
    },
    isGroupOn(key) {
      return this.selectedGroups.includes(key);
    },
    toggleGroup(key) {
      const idx = this.selectedGroups.indexOf(key);
      if (idx >= 0) {
        this.selectedGroups.splice(idx, 1);
      } else {
        this.selectedGroups.push(key);
      }
    },
    selectAllGroups() {
      this.selectedGroups = this.chipGroups.map(g => g.valueKey);
    },
    selectNoGroups() {
      this.selectedGroups = [];
    },
    selectLimitGroups() {
      this.selectedGroups = this.qualityGroups
        .filter(g => this.isJudgeable(g))
        .map(g => g.valueKey);
    },
    cell(value) {
      if (value === null || value === undefined || value === '') {
        return '—';
      }
      const text = String(value);
      return text.length > 60 ? text.slice(0, 60) + '…' : text;
    },
    fullText(value) {
      return value === null || value === undefined ? '' : String(value);
    },
    toNum(value) {
      if (value === null || value === undefined || value === '') {
        return null;
      }
      const n = Number(value);
      return Number.isFinite(n) ? n : null;
    },
    /** 该项是否真的可判定: 有有效上下限, 或属于 OK/NG 判定项 */
    isJudgeable(group) {
      if (group.judgeable !== undefined) {
        return group.judgeable;
      }
      return !!(group.hasLimit || group.booleanCheck);
    },
    limitText(row, g) {
      if (g.hasLimit) {
        const upper = row.values[g.upperKey];
        const lower = row.values[g.lowerKey];
        const fu = this.toNum(upper);
        const fl = this.toNum(lower);
        // 0.0/0.0 是设备占位, 不显示; 上限=下限但非 0 是合法的"固定值"限值, 要显示
        if (fu !== null && fl !== null && !(fu === 0 && fl === 0)) {
          return '限 ' + this.cell(lower) + ' ~ ' + this.cell(upper);
        }
      }
      if (g.booleanCheck) {
        return '0=NG 1=OK';
      }
      return '';
    }
  },
  watch: {
    showAllItems(value) {
      if (value) {
        return;
      }
      // 收起"全部项"时, 把已选中的不可判定项一并移出表格(只影响质量页)
      const groups = this.qualityGroups;
      this.selectedGroups = this.selectedGroups.filter((key) => {
        const group = groups.find(g => g.valueKey === key);
        return group ? this.isJudgeable(group) : false;
      });
      if (!this.selectedGroups.length) {
        this.selectedGroups = this.defaultSelection('quality');
      }
    },
    onlyOut() { this.pageIndex = 0; },
    pageSize() {
      this.pageIndex = 0;
      this.spcPage = 0;
    },
    safePageIndex(value) { this.jumpPageNo = value + 1; },
    safeSpcPage(value) { this.spcJumpPageNo = value + 1; },
    selectedGroups: {
      deep: true,
      handler() { this.pageIndex = 0; }
    }
  },
  mounted() {
    this.refresh(true);
    this.onDocumentClick = (event) => {
      if (!event.target.closest('.combo, .combo-list')) {
        this.deviceOpen = false;
      }
    };
    document.addEventListener('click', this.onDocumentClick);
    setInterval(() => { this.now = Date.now(); }, 1000);
    setInterval(() => {
      if (this.view !== 'env') {
        return;
      }
      if (this.countdown > 0) {
        this.countdown -= 1;
      }
      if (this.countdown <= 0) {
        this.countdown = this.refreshSeconds;
        this.refresh(false);
      }
    }, 1000);
  }
}).mount('#app');
