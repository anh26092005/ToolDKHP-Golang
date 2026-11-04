/** Event registration on Cloudflare: drafts and a single cancellable browser loop. */
const EVENT_FORM_DEFAULTS = Object.freeze({
    preset: 'student-festival-2026', eventId: '', festivalDateId: '183', dateId: '',
    objectId: '12', retries: '10', delayMin: '100', delayMax: '150', timer: ''
});
const EVENT_FESTIVAL_DATES = Object.freeze({
    183: '16/11/2026', 184: '17/11/2026', 185: '18/11/2026'
});
const EVENT_FORM_FIELDS = {
    preset: 'event-preset-select', eventId: 'event-id-input', dateId: 'event-date-id-input',
    objectId: 'event-object-id-input', retries: 'event-retry-input',
    delayMin: 'event-delay-min-input', delayMax: 'event-delay-max-input', timer: 'event-timer-input'
};
let eventAutoRun = null;
let eventRequestInFlight = null;
let eventManualBusy = false;

function getEventDraftKey() {
    const account = api.studentInfo?.maSinhVien || api.getCredentials()?.username;
    return account ? `dkhp_event_form_state:${encodeURIComponent(account)}` : null;
}

function readEventForm() {
    const state = {};
    for (const [key, id] of Object.entries(EVENT_FORM_FIELDS)) {
        state[key] = document.getElementById(id).value.trim();
    }
    state.festivalDateId = document.querySelector('input[name="event-festival-date"]:checked')?.value || '';
    return state;
}

function saveEventFormState() {
    try {
        const key = getEventDraftKey();
        if (key) sessionStorage.setItem(key, JSON.stringify(readEventForm()));
    } catch (_) { /* Registration still works when browser storage is unavailable. */ }
}

function getEventDelayRange(min, max) {
    const delayMin = api.parsePositiveInteger(min, 'Delay Min (ms)');
    const delayMax = api.parsePositiveInteger(max, 'Delay Max (ms)');
    if (delayMin > delayMax) throw new Error('Delay Min phải nhỏ hơn hoặc bằng Delay Max (ms)');
    return { delayMin, delayMax };
}

function restoreEventFormState() {
    let state = { ...EVENT_FORM_DEFAULTS };
    let saved = null;
    try {
        const key = getEventDraftKey();
        saved = key && JSON.parse(sessionStorage.getItem(key) || 'null');
        if (saved && typeof saved === 'object') state = { ...state, ...saved };
    } catch (_) { }
    if (!['student-festival-2026', 'other'].includes(state.preset)) state.preset = EVENT_FORM_DEFAULTS.preset;
    if (!Object.hasOwn(EVENT_FESTIVAL_DATES, state.festivalDateId)) state.festivalDateId = EVENT_FORM_DEFAULTS.festivalDateId;
    try {
        getEventDelayRange(saved?.delayMin, saved?.delayMax);
    } catch (_) {
        state.delayMin = EVENT_FORM_DEFAULTS.delayMin;
        state.delayMax = EVENT_FORM_DEFAULTS.delayMax;
    }
    try {
        api.parsePositiveInteger(state.retries, 'Số lượt thử');
    } catch (_) {
        state.retries = EVENT_FORM_DEFAULTS.retries;
    }
    for (const [key, id] of Object.entries(EVENT_FORM_FIELDS)) {
        document.getElementById(id).value = String(state[key] ?? EVENT_FORM_DEFAULTS[key]);
    }
    document.querySelectorAll('input[name="event-festival-date"]').forEach(radio => {
        radio.checked = radio.value === String(state.festivalDateId);
    });
    updateEventForm();
}

function setupEventForm() {
    const form = document.getElementById('tab-events');
    form.addEventListener('input', () => { updateEventForm(); saveEventFormState(); });
    form.addEventListener('change', () => { updateEventForm(); saveEventFormState(); });
    window.addEventListener('pagehide', stopAutoEvent);
    updateEventForm();
}

// Both manual and automatic registration use this validation/payload builder.
function getEventData(silent = false) {
    try {
        const state = readEventForm();
        const festival = state.preset === 'student-festival-2026';
        const dateId = festival ? state.festivalDateId : state.dateId;
        if (festival) {
            if (!Object.hasOwn(EVENT_FESTIVAL_DATES, dateId)) throw new Error('Vui lòng chọn một ngày tham gia');
            api.parsePositiveInteger(state.objectId, 'objectId');
        }
        return api.buildEventRegistration(state.eventId, dateId, state.objectId);
    } catch (error) {
        if (!silent) showToast(escapeHtml(error.message), 'error');
        return null;
    }
}

function updateEventForm() {
    const festival = document.getElementById('event-preset-select').value === 'student-festival-2026';
    document.getElementById('event-date-cards').hidden = !festival;
    document.getElementById('event-manual-date-group').hidden = festival;
    document.getElementById('event-object-optional').hidden = festival;
    const data = getEventData(true);
    document.getElementById('event-endpoint-preview').textContent =
        `POST https://portal.ut.edu.vn/api/v1${data ? data.endpoint : '/eventWeb/register?eventId=<nhập eventId hợp lệ>'}`;
    document.getElementById('event-payload-preview').textContent = data
        ? JSON.stringify(data.payload, null, 2) : 'Chưa có payload hợp lệ.';
    document.getElementById('event-validation').textContent = data
        ? 'eventId nằm trong query string; các ID có trong JSON body là số nguyên dương.'
        : 'Nhập eventId và các ID bắt buộc hợp lệ để xem yêu cầu đăng ký.';
}

function safeEventMessage(message) {
    let text = String(message || 'Portal không cung cấp thông báo');
    const credentials = api.getCredentials();
    for (const secret of [api.token, credentials?.password]) {
        if (secret) text = text.split(secret).join('[đã ẩn]');
    }
    return text.replace(/\bBearer\s+[A-Za-z0-9._~+/-]+=*/gi, 'Bearer [đã ẩn]')
        .replace(/\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b/g, '[đã ẩn]')
        .replace(/((?:access_token|refresh_token|token|password|mật khẩu)["']?\s*[:=]\s*["']?)[^\s"',;<>}]+/gi, '$1[đã ẩn]');
}

function addEventLog(message, type = 'info') {
    addLog('event-log', escapeHtml(safeEventMessage(message)), type);
}

function setEventStatus(text) {
    document.getElementById('event-run-status').textContent = text;
    updateStatus(text);
}

function updateEventButtons() {
    document.getElementById('event-auto-btn').disabled = !!eventAutoRun || eventManualBusy;
    document.getElementById('event-stop-btn').disabled = !eventAutoRun;
    document.getElementById('event-register-btn').disabled = !!eventAutoRun || !!eventRequestInFlight || eventManualBusy;
    document.getElementById('event-cancel-btn').disabled = !!eventAutoRun || !!eventRequestInFlight || eventManualBusy;
}

function isCurrentEventRun(run) {
    return eventAutoRun === run;
}

// Keep the in-flight request serialized even across Stop -> Start.
async function sendEventRequest(action, run = null) {
    while (eventRequestInFlight) {
        await eventRequestInFlight;
        if (run && !isCurrentEventRun(run)) return null;
    }
    if (run && !isCurrentEventRun(run)) return null;
    const pending = (async () => {
        try {
            return await action();
        } catch (_) {
            return { success: false, isNetworkError: true, message: 'Không thể hoàn tất yêu cầu; vui lòng kiểm tra kết nối' };
        }
    })();
    eventRequestInFlight = pending;
    updateEventButtons();
    try {
        return await pending;
    } finally {
        if (eventRequestInFlight === pending) eventRequestInFlight = null;
        updateEventButtons();
    }
}

async function handleRegisterEvent() {
    if (eventAutoRun || eventRequestInFlight || eventManualBusy) return;
    const data = getEventData();
    if (!data) return;
    saveEventFormState();
    eventManualBusy = true;
    setEventStatus(`Đang đăng ký sự kiện ${data.eventId}...`);
    addEventLog(`Đăng ký một lần: eventId=${data.eventId}, payload=${JSON.stringify(data.payload)}`);
    try {
        const result = await sendEventRequest(() => api.registerEvent(data.eventId, data.payload.dateId, data.payload.objectId));
        const success = result.success === true;
        addEventLog(`${success ? '✅' : '❌'} ${result.message}`, success ? 'success' : 'error');
        showToast(escapeHtml(safeEventMessage(result.message)), success ? 'success' : 'error');
        setEventStatus(success ? 'Đăng ký một lần thành công' : 'Đăng ký một lần chưa thành công');
    } finally {
        eventManualBusy = false;
        updateEventButtons();
    }
}

async function handleCancelEvent() {
    if (eventAutoRun || eventRequestInFlight || eventManualBusy) return;
    let eventId;
    try {
        // Cancellation only needs eventId, regardless of the selected date/object.
        eventId = api.parsePositiveInteger(document.getElementById('event-id-input').value, 'eventId');
    } catch (error) {
        showToast(escapeHtml(error.message), 'error');
        return;
    }
    if (!confirm(`Bạn có chắc muốn hủy đăng ký sự kiện ID: ${eventId}?`)) return;
    eventManualBusy = true;
    setEventStatus(`Đang hủy đăng ký sự kiện ${eventId}...`);
    addEventLog(`Đang hủy đăng ký eventId=${eventId}`);
    try {
        const result = await sendEventRequest(() => api.cancelEvent(eventId));
        const success = result.success === true;
        addEventLog(`${success ? '✅' : '❌'} ${result.message}`, success ? 'success' : 'error');
        showToast(escapeHtml(safeEventMessage(result.message)), success ? 'success' : 'error');
        setEventStatus(success ? 'Đã hủy đăng ký' : 'Hủy đăng ký chưa thành công');
    } finally {
        eventManualBusy = false;
        updateEventButtons();
    }
}

function clearEventLog() {
    document.getElementById('event-log').innerHTML = '';
}

function getEventAutoSettings() {
    const state = readEventForm();
    const retries = api.parsePositiveInteger(state.retries, 'Số lượt thử');
    const range = getEventDelayRange(state.delayMin, state.delayMax);
    let targetTime = null;
    if (state.timer) {
        if (!/^([01]\d|2[0-3]):[0-5]\d(?::[0-5]\d)?$/.test(state.timer)) throw new Error('Giờ hẹn không hợp lệ');
        const now = new Date();
        const [hours, minutes, seconds = 0] = state.timer.split(':').map(Number);
        const target = new Date(now.getFullYear(), now.getMonth(), now.getDate(), hours, minutes, seconds);
        if (target <= now) target.setDate(target.getDate() + 1);
        targetTime = target.getTime();
    }
    return { retries, ...range, targetTime };
}

function startAutoEvent() {
    if (eventAutoRun || eventManualBusy) return;
    const data = getEventData();
    if (!data) return;
    let settings;
    try {
        settings = getEventAutoSettings();
    } catch (error) {
        showToast(escapeHtml(error.message), 'error');
        return;
    }
    saveEventFormState();
    const run = {
        data: Object.freeze({ ...data, payload: Object.freeze({ ...data.payload }) }),
        ...settings, attempts: 0, requests: 0, timeout: null, cancelWait: null
    };
    eventAutoRun = run;
    document.getElementById('event-run-config').textContent =
        `Cấu hình đang chạy: eventId=${data.eventId}, payload=${JSON.stringify(data.payload)}, delay=${settings.delayMin}–${settings.delayMax} ms, tối đa ${settings.retries} lượt.`;
    updateEventButtons();
    return runLocalEventLoop(run);
}

// Every wait belongs to one run. Stop resolves it and invalidates old callbacks.
function waitForEventRun(run, milliseconds) {
    return new Promise(resolve => {
        if (!isCurrentEventRun(run)) return resolve(false);
        let remaining = milliseconds;
        const finish = value => {
            if (run.timeout !== null) clearTimeout(run.timeout);
            run.timeout = null;
            run.cancelWait = null;
            resolve(value);
        };
        run.cancelWait = () => finish(false);
        const schedule = () => {
            // Avoid the browser's signed 32-bit timer overflow for large user values.
            const chunk = Math.min(remaining, 2147483647);
            run.timeout = setTimeout(() => {
                if (!isCurrentEventRun(run)) return finish(false);
                remaining -= chunk;
                if (remaining > 0) schedule();
                else finish(true);
            }, chunk);
        };
        schedule();
    });
}

function finishEventRun(run, status) {
    if (!isCurrentEventRun(run)) return;
    eventAutoRun = null;
    if (run.cancelWait) run.cancelWait();
    updateEventButtons();
    setEventStatus(status);
}

function stopAutoEvent() {
    const run = eventAutoRun;
    if (!run) return;
    finishEventRun(run, `Đã dừng (${run.attempts}/${run.retries} lượt nghiệp vụ; ${run.requests} request)${eventRequestInFlight ? ' — chờ phản hồi request hiện tại' : ''}`);
    addEventLog('🛑 Đã dừng tự động; đã hủy lịch chờ và không gửi thêm lượt của lần chạy này.', 'warning');
}

async function runLocalEventLoop(run) {
    if (run.targetTime !== null && run.targetTime > Date.now()) {
        const time = new Date(run.targetTime).toLocaleString('vi-VN');
        addEventLog(`⏳ Hẹn giờ trong tab đến ${time}. Giữ tab và trình duyệt hoạt động.`);
        setEventStatus(`Đang chờ ${time} — 0/${run.retries} lượt`);
        if (!await waitForEventRun(run, run.targetTime - Date.now())) return;
    }
    if (!isCurrentEventRun(run)) return;
    addEventLog(`🚀 Bắt đầu tự động: eventId=${run.data.eventId}, payload=${JSON.stringify(run.data.payload)}`);
    let consecutiveErrors = 0;
    while (isCurrentEventRun(run) && run.attempts < run.retries) {
        const result = await sendEventRequest(() => {
            run.requests++;
            setEventStatus(`Đang gửi lượt ${run.attempts + 1}/${run.retries} — request ${run.requests} (tối đa 20 giây/yêu cầu)...`);
            addEventLog(`🔄 Lượt ${run.attempts + 1}/${run.retries} — request ${run.requests}`);
            return api.registerEvent(run.data.eventId, run.data.payload.dateId, run.data.payload.objectId,
                () => isCurrentEventRun(run), message => {
                    if (!isCurrentEventRun(run)) return;
                    setEventStatus(message);
                    addEventLog(message, 'warning');
                });
        }, run);
        if (!isCurrentEventRun(run)) return;
        if (result.isAuthenticationFailed || (result.isUnauthorized && !api.getCredentials())) {
            addEventLog(`❌ ${result.message} Cần đăng nhập lại để tiếp tục.`, 'error');
            finishEventRun(run, 'Cần đăng nhập lại — kiểm tra tài khoản/mật khẩu và bật Ghi nhớ');
            return;
        }
        const transient = result.isUnauthorized || result.isServerError || result.isNetworkError || result.isRateLimit || result.statusCode >= 500;
        if (!transient) run.attempts++;
        if (result.success === true) {
            addEventLog(`✅ THÀNH CÔNG: ${result.message}`, 'success');
            finishEventRun(run, `Đăng ký thành công sau ${run.attempts}/${run.retries} lượt`);
            showToast('Đăng ký sự kiện thành công!', 'success');
            playSuccessSound();
            return;
        }
        addEventLog(`❌ Lượt ${run.attempts}: ${result.message}`, 'error');
        if (run.attempts >= run.retries) break;
        if (transient) {
            consecutiveErrors++;
            const baseBackoff = result.isUnauthorized ? 5000 : Math.min(2000 + consecutiveErrors * 500, 5000);
            const backoff = Math.max(baseBackoff + Math.floor(Math.random() * 501), result.retryAfterMs || 0);
            addEventLog(`⚠️ Backoff lỗi Portal/phiên/kết nối: ${backoff} ms. Giữ nguyên lượt thử; sẽ tự tiếp tục khi Portal hồi phục.`, 'warning');
            setEventStatus(`Portal/phiên chưa sẵn sàng — thử lại sau ${backoff} ms (${run.attempts}/${run.retries} lượt nghiệp vụ)`);
            if (!await waitForEventRun(run, backoff)) return;
        } else {
            consecutiveErrors = 0;
        }
        const { delayMin, delayMax } = run;
        const delay = Math.floor(Math.random() * (delayMax - delayMin + 1)) + delayMin;
        addEventLog(`⏳ Nghỉ ${delay} ms sau request trước.`, 'warning');
        setEventStatus(`Đang chờ ${delay} ms — ${run.attempts}/${run.retries} lượt`);
        if (!await waitForEventRun(run, delay)) return;
    }
    if (isCurrentEventRun(run)) {
        addEventLog(`🛑 Đã hết ${run.retries} lượt thử.`, 'warning');
        finishEventRun(run, `Đã hết lượt thử (${run.attempts}/${run.retries})`);
    }
}
