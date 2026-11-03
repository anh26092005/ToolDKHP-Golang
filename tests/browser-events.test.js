const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

const html = fs.readFileSync('public/index.html', 'utf8');
const apiSource = fs.readFileSync('public/js/api.js', 'utf8');
const eventsSource = fs.readFileSync('public/js/events.js', 'utf8');
const plain = value => JSON.parse(JSON.stringify(value));
const deferred = () => {
    let resolve;
    const promise = new Promise(done => { resolve = done; });
    return { promise, resolve };
};
async function flush() {
    for (let i = 0; i < 50; i++) await Promise.resolve();
}
function storage(values = new Map()) {
    return { values, getItem: key => values.get(key) ?? null,
        setItem: (key, value) => values.set(key, String(value)), removeItem: key => values.delete(key) };
}

function browser({ reply = () => ({ success: false, message: 'Chưa mở đăng ký' }), session = storage(), random = () => 0, fetch } = {}) {
    const nodes = new Map([...html.matchAll(/id="([^"]+)"/g)].map(match => [match[1], {
        value: '', textContent: '', innerHTML: '', disabled: false, hidden: false, addEventListener() {}
    }]));
    for (const match of html.matchAll(/<input\b[^>]*\bid="([^"]+)"[^>]*>/g)) {
        const tag = match[0];
        Object.assign(nodes.get(match[1]), { value: tag.match(/\bvalue="([^"]*)"/)?.[1] || '', disabled: /\bdisabled\b/.test(tag) });
    }
    nodes.get('event-preset-select').value = 'student-festival-2026';
    const radios = [...html.matchAll(/<input\b[^>]*name="event-festival-date"[^>]*>/g)].map(match => ({
        value: match[0].match(/value="(\d+)"/)[1], checked: /\bchecked\b/.test(match[0])
    }));
    const now = new Date(2026, 10, 3, 8, 0, 0).getTime();
    const clock = { now, timers: new Map(), history: [], next: 1 };
    const requests = [], logs = [], toasts = [];
    const local = storage(new Map([['dkhp_token', 'test-token'], ['dkhp_student', '{"maSinhVien":"student-A"}']]));
    const context = {
        localStorage: local, sessionStorage: session, window: { addEventListener() {}, location: {} },
        document: {
            getElementById: id => nodes.get(id),
            querySelector: () => radios.find(radio => radio.checked), querySelectorAll: () => radios
        },
        Date: class extends Date {
            constructor(...args) { super(...(args.length ? args : [clock.now])); }
            static now() { return clock.now; }
        },
        Math: Object.assign(Object.create(Math), { random }),
        setTimeout(callback, ms) {
            const timer = { id: clock.next++, callback, ms, due: clock.now + ms };
            clock.timers.set(timer.id, timer);
            clock.history.push(timer);
            return timer.id;
        },
        clearTimeout: id => clock.timers.delete(id),
        setInterval() { throw new Error('Event requests must not use setInterval'); },
        addLog: (container, message, type) => logs.push({ container, message, type }),
        showToast: (message, type) => toasts.push({ message, type }),
        escapeHtml: text => String(text).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#039;'),
        updateStatus() {}, playSuccessSound() {}, confirm: () => true,
        AbortSignal, Response, console: { log() {}, warn() {}, error() {} },
        btoa, atob, encodeURIComponent, decodeURIComponent, escape, unescape,
        fetch: fetch || (async (url, options) => {
            assert.equal(url, '/api/gateway', 'Event actions only use the Cloudflare gateway');
            const payload = JSON.parse(Buffer.from(JSON.parse(options.body).p, 'base64').toString());
            requests.push({ ...payload, at: clock.now });
            const result = await reply(payload, requests.length);
            return result instanceof Response ? result : new Response(JSON.stringify({ r: Buffer.from(JSON.stringify(result)).toString('base64') }));
        })
    };
    vm.createContext(context);
    vm.runInContext(apiSource + '\n' + eventsSource + '\nthis.client = api;', context);
    return {
        context, nodes, radios, clock, requests, logs, toasts, local, session,
        value: (id, value) => { nodes.get(id).value = String(value); },
        date: id => radios.forEach(radio => { radio.checked = radio.value === String(id); }),
        async advance(ms) {
            const end = clock.now + ms;
            for (;;) {
                const next = [...clock.timers.values()].filter(timer => timer.due <= end).sort((a, b) => a.due - b.due)[0];
                if (!next) break;
                clock.now = next.due;
                clock.timers.delete(next.id);
                next.callback();
                await flush();
            }
            clock.now = end;
            await flush();
        }
    };
}

test('all three festival cards register through the real Cloudflare proxy with integer payloads', async () => {
    const worker = fs.readFileSync('functions/api/[[path]].js', 'utf8');
    const { onRequest } = await import(`data:text/javascript;base64,${Buffer.from(worker).toString('base64')}`);
    const originalFetch = global.fetch;
    const upstream = [];
    global.fetch = async (url, options) => {
        upstream.push({ url, options });
        return new Response('{"success":true,"message":"Đã đăng ký"}');
    };
    try {
        const h = browser({ fetch: (url, options) => onRequest({
            request: new Request('https://example.workers.dev' + url, options), params: { path: ['gateway'] }, env: {}
        }) });
        assert.equal(h.nodes.get('event-id-input').value, '');
        assert.equal(h.nodes.get('event-object-id-input').value, '12');
        assert.deepEqual(h.radios.map(radio => radio.value), ['183', '184', '185']);
        for (const [dateId, label] of [[183, '16/11/2026'], [184, '17/11/2026'], [185, '18/11/2026']]) {
            assert.ok(html.includes(`<strong>${label}</strong><small>dateId = ${dateId}</small>`));
            h.value('event-id-input', '777'); // The preset never supplies eventId 114.
            h.date(dateId);
            h.context.updateEventForm();
            await h.context.handleRegisterEvent();
            const call = upstream.at(-1);
            assert.equal(call.url, 'https://portal.ut.edu.vn/api/v1/eventWeb/register?eventId=777');
            assert.equal(call.options.method, 'POST');
            assert.equal(call.options.headers.get('Authorization'), 'Bearer test-token');
            assert.deepEqual(JSON.parse(call.options.body), { dateId, objectId: 12 });
            assert.equal(h.nodes.get('event-endpoint-preview').textContent, 'POST ' + call.url);
            assert.equal(h.nodes.get('event-payload-preview').textContent, JSON.stringify({ dateId, objectId: 12 }, null, 2));
        }
    } finally { global.fetch = originalFetch; }
});

test('manual and auto share validation, respect edited objectId and snapshot all IDs/settings', async () => {
    const h = browser({ reply: (_, count) => ({ success: count >= 3, message: 'Kết quả' }) });
    h.value('event-id-input', 114);
    h.value('event-object-id-input', 27);
    h.value('event-retry-input', 2);
    h.date(184);
    await h.context.handleRegisterEvent();
    const running = h.context.startAutoEvent();
    h.context.startAutoEvent();
    h.context.startAutoEvent();
    await flush();
    assert.equal(h.requests.length, 2);
    h.value('event-id-input', 999);
    h.value('event-object-id-input', 99);
    h.value('event-delay-min-input', 5000);
    h.value('event-delay-max-input', 6000);
    h.value('event-retry-input', 50);
    h.date(185);
    await h.advance(100);
    await running;
    assert.equal(h.requests.length, 3);
    for (const request of h.requests) {
        assert.equal(request.endpoint, '/eventWeb/register?eventId=114');
        assert.deepEqual(request.data, { dateId: 184, objectId: 27 });
    }
    assert.equal(h.requests[2].at - h.requests[1].at, 100);
    assert.match(h.nodes.get('event-run-status').textContent, /thành công.*2\/2/);
    assert.equal(h.nodes.get('event-auto-btn').disabled, false);
    assert.equal(h.nodes.get('event-stop-btn').disabled, true);
    await h.context.startAutoEvent(); // New run picks up the edited values and succeeds immediately.
    assert.equal(h.requests.at(-1).endpoint, '/eventWeb/register?eventId=999');
    assert.deepEqual(h.requests.at(-1).data, { dateId: 185, objectId: 99 });
});

test('eventId stays empty across preset changes; invalid IDs cannot reach either registration path', async () => {
    const h = browser();
    for (const preset of ['other', 'student-festival-2026']) {
        h.value('event-preset-select', preset);
        h.context.updateEventForm();
        assert.equal(h.nodes.get('event-id-input').value, '');
    }
    for (const id of ['', '0', '-1', '1.5', '1e2', '12junk', '9007199254740992']) {
        h.value('event-id-input', id);
        await h.context.handleRegisterEvent();
        await h.context.startAutoEvent();
    }
    h.value('event-id-input', 114);
    for (const object of ['', 0, -1, 1.5, '12junk']) {
        h.value('event-object-id-input', object);
        await h.context.handleRegisterEvent();
        await h.context.startAutoEvent();
    }
    assert.equal(h.requests.length, 0);
    assert.equal(h.clock.timers.size, 0);
});

test('other events allow either optional ID to be omitted and keep edited objectId on preset changes', async () => {
    const h = browser();
    h.value('event-id-input', 302);
    h.value('event-preset-select', 'other');
    h.value('event-object-id-input', '');
    await h.context.handleRegisterEvent();
    assert.deepEqual(h.requests.at(-1).data, {});
    h.value('event-date-id-input', 123);
    await h.context.handleRegisterEvent();
    assert.deepEqual(h.requests.at(-1).data, { dateId: 123 });
    h.value('event-date-id-input', '');
    h.value('event-object-id-input', 34);
    await h.context.handleRegisterEvent();
    assert.deepEqual(h.requests.at(-1).data, { objectId: 34 });
    h.value('event-preset-select', 'student-festival-2026');
    h.context.updateEventForm();
    assert.equal(h.nodes.get('event-object-id-input').value, '34');
    assert.equal(h.nodes.get('event-manual-date-group').hidden, true);
    assert.deepEqual(plain(h.context.getEventData()).payload, { dateId: 183, objectId: 34 });
    h.value('event-preset-select', 'other');
    h.value('event-date-id-input', '2.5');
    await h.context.handleRegisterEvent();
    await h.context.startAutoEvent();
    assert.equal(h.requests.length, 3);
});

test('100 and 150 ms are actual post-response delays, without overlapping slow requests', async () => {
    const first = deferred();
    let randomCalls = 0;
    const h = browser({ random: () => randomCalls++ === 0 ? 0 : 0.999999,
        reply: (_, count) => count === 1 ? first.promise : { success: false, message: 'Chưa mở' } });
    h.value('event-id-input', 114);
    h.value('event-retry-input', 3);
    const running = h.context.startAutoEvent();
    await h.advance(1200);
    assert.equal(h.requests.length, 1);
    assert.equal(h.clock.timers.size, 0, 'No next timer exists before the request finishes');
    first.resolve({ success: false, message: 'Chưa mở' });
    await flush();
    assert.equal(h.clock.history.at(-1).ms, 100);
    await h.advance(99);
    assert.equal(h.requests.length, 1);
    await h.advance(1);
    assert.equal(h.requests.length, 2);
    assert.equal(h.requests[1].at, h.clock.now);
    assert.equal(h.clock.history.at(-1).ms, 150);
    await h.advance(149);
    assert.equal(h.requests.length, 2);
    await h.advance(1);
    await running;
    assert.equal(h.requests.length, 3);
    assert.equal(h.clock.timers.size, 0);
    assert.match(h.nodes.get('event-run-status').textContent, /hết lượt.*3\/3/);
});

test('delay/retry validation rejects blanks, fractions, nonpositive numbers and reversed ranges', async () => {
    const h = browser();
    h.value('event-id-input', 114);
    for (const [min, max] of [['', 150], [0, 150], [-1, 150], [100.5, 150], ['1e2', 150], [151, 150], [100, ''], [100, 0]]) {
        h.value('event-delay-min-input', min);
        h.value('event-delay-max-input', max);
        await h.context.startAutoEvent();
    }
    assert.equal(h.requests.length, 0);
    h.value('event-delay-min-input', 1);
    h.value('event-delay-max-input', 1);
    for (const retries of ['', 0, -1, 1.5]) {
        h.value('event-retry-input', retries);
        await h.context.startAutoEvent();
    }
    assert.equal(h.requests.length, 0);
    h.value('event-retry-input', 2);
    const running = h.context.startAutoEvent();
    await flush();
    assert.equal(h.clock.history.at(-1).ms, 1);
    await h.advance(1);
    await running;
    assert.equal(h.requests.length, 2);
});

test('Stop cancels delay and stale callbacks cannot send or affect the next run', async () => {
    const h = browser();
    h.value('event-id-input', 114);
    const old = h.context.startAutoEvent();
    await flush();
    const stale = h.clock.history.at(-1).callback;
    h.context.stopAutoEvent();
    assert.equal(h.clock.timers.size, 0);
    h.value('event-id-input', 555);
    const current = h.context.startAutoEvent();
    await flush();
    stale();
    await flush();
    assert.equal(h.requests.length, 2);
    assert.equal(h.nodes.get('event-auto-btn').disabled, true);
    h.context.stopAutoEvent();
    await h.advance(10000);
    await Promise.all([old, current]);
    assert.equal(h.requests.length, 2);
});

test('Stop -> Start waits for the old in-flight request and ignores its late success', async () => {
    const first = deferred();
    const h = browser({ reply: (_, count) => count === 1 ? first.promise : { success: false, message: 'Chưa mở' } });
    h.value('event-id-input', 114);
    const old = h.context.startAutoEvent();
    h.context.stopAutoEvent();
    h.value('event-id-input', 556);
    const current = h.context.startAutoEvent();
    h.context.startAutoEvent();
    await h.advance(10000);
    assert.equal(h.requests.length, 1);
    first.resolve({ success: true, message: 'Kết quả cũ' });
    await flush();
    assert.equal(h.requests.length, 2);
    assert.equal(h.requests[1].endpoint, '/eventWeb/register?eventId=556');
    assert.equal(h.nodes.get('event-auto-btn').disabled, true);
    assert.ok(!h.logs.some(log => log.message.includes('Kết quả cũ')));
    h.context.stopAutoEvent();
    await Promise.all([old, current]);
});

test('scheduled start uses the original snapshot once; cancelled schedule cannot fire later', async () => {
    const h = browser({ reply: () => ({ success: true }) });
    h.value('event-id-input', 114);
    h.value('event-timer-input', '08:00:10');
    h.date(184);
    const scheduled = h.context.startAutoEvent();
    h.context.startAutoEvent();
    h.value('event-object-id-input', 77);
    h.date(185);
    assert.equal(h.clock.timers.size, 1);
    await h.advance(9999);
    assert.equal(h.requests.length, 0);
    await h.advance(1);
    await scheduled;
    assert.equal(h.requests.length, 1);
    assert.deepEqual(h.requests[0].data, { dateId: 184, objectId: 12 });
    h.value('event-timer-input', '08:00:20');
    const cancelled = h.context.startAutoEvent();
    const stale = h.clock.history.at(-1).callback;
    h.context.stopAutoEvent();
    stale();
    await h.advance(20000);
    await cancelled;
    assert.equal(h.requests.length, 1);
    h.value('event-timer-input', '07:59:59');
    const tomorrow = h.context.startAutoEvent();
    assert.ok(h.clock.history.at(-1).ms > 23 * 60 * 60 * 1000);
    h.context.stopAutoEvent();
    await tomorrow;
});

test('only explicit Portal success:true stops auto, including when message contains the word lỗi', async () => {
    const results = [{ success: 'true' }, { message: 'Unknown' }, new Response(''), { success: true, message: 'Không có lỗi' }];
    const h = browser({ reply: (_, count) => results[count - 1] });
    h.value('event-id-input', 114);
    const running = h.context.startAutoEvent();
    await flush();
    await h.advance(300);
    await running;
    assert.equal(h.requests.length, 4);
    assert.equal(h.clock.timers.size, 0);
    assert.match(h.nodes.get('event-run-status').textContent, /thành công/);
});

test('Portal failures preserve error flags and use a separate cancellable backoff before normal delay', async () => {
    for (const status of [429, 500, 502, 503]) {
        const h = browser({ reply: () => new Response('{}', { status }) });
        h.value('event-id-input', 114);
        const running = h.context.startAutoEvent();
        await flush();
        assert.equal(h.clock.history.at(-1).ms, 2500);
        await h.advance(2500);
        assert.equal(h.requests.length, 1);
        assert.equal(h.clock.history.at(-1).ms, 100);
        await h.advance(100);
        assert.equal(h.requests.length, 2);
        h.context.stopAutoEvent();
        await running;
        assert.equal(h.clock.timers.size, 0);
    }
});

test('event auth refresh retries the same payload; Stop during refresh prevents an old retry', async () => {
    for (const stopDuringRefresh of [false, true]) {
        const login = deferred();
        const requests = [];
        const h = browser({ fetch: async (url, options) => {
            if (url === '/api/user/login') return login.promise;
            assert.equal(url, '/api/gateway');
            const payload = JSON.parse(Buffer.from(JSON.parse(options.body).p, 'base64').toString());
            requests.push(payload);
            return requests.length === 1 ? new Response('{}', { status: 401 })
                : new Response('{"success":true,"message":"Refreshed"}');
        } });
        h.context.client.saveCredentials('student-A', 'test-password');
        h.value('event-id-input', 114);
        h.date(184);
        const running = h.context.startAutoEvent();
        await flush();
        if (stopDuringRefresh) h.context.stopAutoEvent();
        login.resolve(new Response('{"success":true,"token":"refreshed-token"}'));
        await running;
        assert.equal(requests.length, stopDuringRefresh ? 1 : 2);
        if (!stopDuringRefresh) {
            assert.equal(requests[1].token, 'refreshed-token');
            assert.deepEqual(requests[1].data, { dateId: 184, objectId: 12 });
        }
    }
});

test('drafts restore edited objectId and valid delays per account/tab; invalid or missing ranges use 100/150', () => {
    const h = browser();
    h.context.restoreEventFormState();
    assert.equal(h.nodes.get('event-id-input').value, '');
    assert.equal(h.nodes.get('event-delay-min-input').value, '100');
    assert.equal(h.nodes.get('event-delay-max-input').value, '150');
    h.value('event-id-input', 114);
    h.value('event-object-id-input', 31);
    h.value('event-delay-min-input', 1000);
    h.value('event-delay-max-input', 2000);
    h.date(185);
    h.context.saveEventFormState();
    const reloaded = browser({ session: h.session });
    reloaded.context.restoreEventFormState();
    assert.deepEqual(plain(reloaded.context.getEventData()).payload, { dateId: 185, objectId: 31 });
    assert.equal(reloaded.nodes.get('event-delay-min-input').value, '1000');
    assert.equal(reloaded.nodes.get('event-delay-max-input').value, '2000');
    reloaded.context.client.studentInfo = { maSinhVien: 'student-B' };
    reloaded.context.restoreEventFormState();
    assert.equal(reloaded.nodes.get('event-id-input').value, '');
    assert.equal(reloaded.nodes.get('event-object-id-input').value, '12');
    reloaded.context.client.studentInfo = { maSinhVien: 'student-A' };
    reloaded.context.restoreEventFormState();
    assert.equal(reloaded.nodes.get('event-object-id-input').value, '31');
    const otherTab = browser();
    otherTab.context.restoreEventFormState();
    assert.equal(otherTab.nodes.get('event-id-input').value, '');
    for (const range of [{}, { delayMin: '20' }, { delayMax: '999' }, { delayMin: '0', delayMax: '150' }, { delayMin: '151', delayMax: '150' }]) {
        h.session.setItem('dkhp_event_form_state:student-A', JSON.stringify({ objectId: '45', ...range }));
        h.context.restoreEventFormState();
        assert.equal(h.nodes.get('event-delay-min-input').value, '100');
        assert.equal(h.nodes.get('event-delay-max-input').value, '150');
        assert.equal(h.nodes.get('event-object-id-input').value, '45');
    }
    h.session.setItem('dkhp_event_form_state:student-A', '{broken');
    h.context.restoreEventFormState();
    assert.equal(h.nodes.get('event-object-id-input').value, '12');
});

test('cancellation only validates eventId and duplicate manual clicks are serialized', async () => {
    const response = deferred();
    const h = browser({ reply: () => response.promise });
    h.value('event-id-input', 114);
    h.value('event-object-id-input', '');
    h.value('event-date-id-input', 'bad');
    const cancel = h.context.handleCancelEvent();
    h.context.handleCancelEvent();
    h.context.handleRegisterEvent();
    h.context.startAutoEvent();
    assert.equal(h.requests.length, 1);
    assert.equal(h.requests[0].method, 'DELETE');
    assert.equal(h.requests[0].endpoint, '/eventWeb/cancel/114');
    response.resolve({ success: true });
    await cancel;
    assert.equal(h.nodes.get('event-auto-btn').disabled, false);
});

test('event logs/toasts redact secrets and escape Portal HTML; drafts contain no credentials', async () => {
    const h = browser({ reply: () => ({ success: false, message: '<img src=x> test-token test-password Bearer other-secret token=unknown-secret' }) });
    h.context.client.saveCredentials('student-A', 'test-password');
    h.value('event-id-input', 114);
    await h.context.handleRegisterEvent();
    const output = JSON.stringify([...h.logs, ...h.toasts]);
    for (const secret of ['test-token', 'test-password', 'other-secret', 'unknown-secret', '<img']) assert.ok(!output.includes(secret));
    assert.ok(output.includes('&lt;img'));
    const draft = h.session.getItem('dkhp_event_form_state:student-A');
    assert.ok(!/token|password|credentials/.test(draft));
});
