const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

test('Step 2 keeps its retry budget and selected classes through a Portal outage', async () => {
    const source = fs.readFileSync('public/js/app.js', 'utf8');
    const start = source.indexOf('async function runStep2LiveAutoRegister()');
    const end = source.indexOf('\nfunction stopStep2AutoRegister()', start);
    const groups = [{ maHocPhan: 'A', tenHocPhan: 'Course', classIds: ['123'], type: 'main', status: 'waiting' }];
    let calls = 0;
    const context = {
        _activeCourseGroups: groups, _step2ModalLoopCount: 1, _step2AutoRegisterRunning: false,
        _spamRequestCount: 0, document: { getElementById: () => null },
        api: { registerMainClass: async () => ++calls < 4
            ? { success: false, isServerError: true, statusCode: 500 }
            : { success: true }, getCredentials: () => ({ username: 'user', password: 'password' }) },
        setTimeout: callback => callback(), addStep2ModalLog() {}, updateCourseSpamMsg() {},
        showToast() {}, getRandomSpamDelay: () => 1
    };
    vm.createContext(context);
    vm.runInContext(source.slice(start, end), context);
    await context.runStep2LiveAutoRegister();
    assert.equal(calls, 4);
    assert.equal(groups[0].status, 'success');
    assert.equal(groups[0].successfulClassId, '123');
});

test('Portal 401 preserves the browser session when relogin fails', async () => {
    const source = fs.readFileSync('public/js/api.js', 'utf8');
    const values = new Map([['dkhp_token', 'old-token']]);
    const storage = { getItem: key => values.get(key) || null, setItem: (key, value) => values.set(key, value), removeItem: key => values.delete(key) };
    const context = {
        localStorage: storage, window: { location: { href: 'unchanged' } },
        fetch: async () => new Response('{"message":"expired"}', { status: 401 }),
        AbortSignal, Response, console, btoa, atob, encodeURIComponent, decodeURIComponent, escape, unescape
    };
    vm.createContext(context);
    vm.runInContext(source + '\nthis.client = api;', context);
    const result = await context.client.request('POST', '/dkhp/test');
    assert.equal(result.isUnauthorized, true);
    assert.equal(values.get('dkhp_token'), 'old-token');
    assert.equal(context.window.location.href, 'unchanged');
});

test('Portal requests resume after a later successful automatic login', async () => {
    const source = fs.readFileSync('public/js/api.js', 'utf8');
    const credentials = btoa(encodeURIComponent(JSON.stringify({ username: 'student', password: 'secret' })));
    const values = new Map([['dkhp_token', 'old-token'], ['dkhp_credentials', credentials]]);
    const storage = { getItem: key => values.get(key) || null, setItem: (key, value) => values.set(key, value), removeItem: key => values.delete(key) };
    let loginCalls = 0;
    const context = {
        localStorage: storage, window: { location: { href: 'unchanged' } },
        fetch: async (url, options) => {
            if (url.endsWith('/user/login')) {
                loginCalls++;
                return loginCalls === 1 ? new Response('{}', { status: 500 })
                    : new Response(JSON.stringify({ success: true, token: 'new-token' }));
            }
            return options.headers.Authorization === 'Bearer new-token'
                ? new Response(JSON.stringify({ success: true }))
                : new Response('{}', { status: 401 });
        },
        AbortSignal, Response, console: { error() {}, warn() {}, log() {} }, btoa, atob, encodeURIComponent, decodeURIComponent, escape, unescape
    };
    vm.createContext(context);
    vm.runInContext(source + '\nthis.client = api;', context);
    assert.equal((await context.client.request('POST', '/dkhp/test')).isUnauthorized, true);
    assert.equal((await context.client.request('POST', '/dkhp/test')).success, true);
    assert.equal(values.get('dkhp_token'), 'new-token');
});

test('Cloudflare gateway and browser recover after a simulated 30-minute Portal outage', async () => {
    const workerSource = fs.readFileSync('functions/api/[[path]].js', 'utf8');
    const { onRequest } = await import(`data:text/javascript;base64,${Buffer.from(workerSource).toString('base64')}`);
    const apiSource = fs.readFileSync('public/js/api.js', 'utf8');
    const appSource = fs.readFileSync('public/js/app.js', 'utf8');
    const start = appSource.indexOf('async function runStep2LiveAutoRegister()');
    const end = appSource.indexOf('\nfunction stopStep2AutoRegister()', start);
    const groups = [{ maHocPhan: 'A', tenHocPhan: 'Course', classIds: ['123'], type: 'main', status: 'waiting' }];
    const values = new Map([['dkhp_token', 'token']]);
    const storage = { getItem: key => values.get(key) || null, setItem: (key, value) => values.set(key, value), removeItem: key => values.delete(key) };
    const originalFetch = global.fetch;
    let elapsed = 0;
    let portalCalls = 0;
    let gatewayCalls = 0;
    try {
        global.fetch = async (url, options) => {
            assert.equal(url, 'https://portal.ut.edu.vn/api/v1/dkhp/dangKyLopHocPhan?idLopHocPhan=123');
            assert.equal(options.headers.get('Authorization'), 'Bearer token');
            portalCalls++;
            return elapsed < 30 * 60 * 1000
                ? new Response(JSON.stringify({ success: false, message: 'Portal unavailable' }), { status: 500 })
                : new Response(JSON.stringify({ success: true, message: 'Registered' }));
        };
        const context = {
            localStorage: storage, window: { location: {} }, AbortSignal, Response, console,
            btoa, atob, encodeURIComponent, decodeURIComponent, escape, unescape,
            fetch: async (url, options) => {
                assert.equal(url, '/api/gateway');
                gatewayCalls++;
                const request = new Request('https://example.workers.dev' + url, options);
                return onRequest({ request, params: { path: ['gateway'] }, env: {} });
            },
            _activeCourseGroups: groups, _step2ModalLoopCount: 1, _step2AutoRegisterRunning: false,
            _spamRequestCount: 0, document: { getElementById: () => null },
            setTimeout: (callback, ms) => { elapsed += ms; callback(); },
            addStep2ModalLog() {}, updateCourseSpamMsg() {}, showToast() {}, getRandomSpamDelay: () => 1
        };
        vm.createContext(context);
        vm.runInContext(apiSource + '\n' + appSource.slice(start, end), context);
        await context.runStep2LiveAutoRegister();
        assert.ok(elapsed >= 30 * 60 * 1000);
        assert.equal(portalCalls, 363);
        assert.equal(gatewayCalls, portalCalls);
        assert.equal(groups[0].status, 'success');
        assert.equal(groups[0].successfulClassId, '123');
    } finally {
        global.fetch = originalFetch;
    }
});

test('60 browser sessions recover independently after a simulated 30-minute outage', async () => {
    const workerSource = fs.readFileSync('functions/api/[[path]].js', 'utf8');
    const { onRequest } = await import(`data:text/javascript;base64,${Buffer.from(workerSource).toString('base64')}`);
    const apiSource = fs.readFileSync('public/js/api.js', 'utf8');
    const appSource = fs.readFileSync('public/js/app.js', 'utf8');
    const start = appSource.indexOf('async function runStep2LiveAutoRegister()');
    const end = appSource.indexOf('\nfunction stopStep2AutoRegister()', start);
    const elapsed = Array(60).fill(0);
    const calls = Array(60).fill(0);
    const originalFetch = global.fetch;
    try {
        global.fetch = async (url, options) => {
            const token = options.headers.get('Authorization');
            const user = Number(token.replace('Bearer user-', ''));
            assert.ok(Number.isInteger(user) && user >= 0 && user < 60);
            assert.equal(new URL(url).searchParams.get('idLopHocPhan'), String(100000 + user));
            calls[user]++;
            return elapsed[user] < 30 * 60 * 1000
                ? new Response('{"success":false}', { status: 500 })
                : new Response('{"success":true}');
        };
        await Promise.all(Array.from({ length: 60 }, async (_, user) => {
            const groups = [{ maHocPhan: 'A', tenHocPhan: 'Course', classIds: [String(100000 + user)], type: 'main', status: 'waiting' }];
            const values = new Map([['dkhp_token', `user-${user}`]]);
            const context = {
                localStorage: { getItem: key => values.get(key) || null, setItem: (key, value) => values.set(key, value), removeItem: key => values.delete(key) },
                window: { location: {} }, AbortSignal, Response, console,
                btoa, atob, encodeURIComponent, decodeURIComponent, escape, unescape,
                fetch: async (url, options) => onRequest({ request: new Request('https://example.workers.dev' + url, options), params: { path: ['gateway'] }, env: {} }),
                _activeCourseGroups: groups, _step2ModalLoopCount: 1, _step2AutoRegisterRunning: false,
                _spamRequestCount: 0, document: { getElementById: () => null },
                setTimeout: (callback, ms) => { elapsed[user] += ms; callback(); },
                addStep2ModalLog() {}, updateCourseSpamMsg() {}, showToast() {}, getRandomSpamDelay: () => 1
            };
            vm.createContext(context);
            vm.runInContext(apiSource + '\n' + appSource.slice(start, end), context);
            await context.runStep2LiveAutoRegister();
            assert.equal(groups[0].status, 'success');
            assert.equal(groups[0].successfulClassId, String(100000 + user));
            assert.equal(calls[user], 363);
        }));
    } finally {
        global.fetch = originalFetch;
    }
});

test('20 logged-in users start one registration each at the same scheduled time', async () => {
    const workerSource = fs.readFileSync('functions/api/[[path]].js', 'utf8');
    const { onRequest } = await import(`data:text/javascript;base64,${Buffer.from(workerSource).toString('base64')}`);
    const apiSource = fs.readFileSync('public/js/api.js', 'utf8');
    const originalFetch = global.fetch;
    let active = 0;
    let maxActive = 0;
    const registered = new Set();
    let release;
    const scheduledTime = new Promise(resolve => { release = resolve; });
    try {
        global.fetch = async (url, options) => {
            const target = new URL(url);
            if (target.pathname.endsWith('/user/login')) {
                const credentials = JSON.parse(Buffer.from(options.body).toString());
                return new Response(JSON.stringify({ success: true, token: `token-${credentials.username}` }));
            }
            if (target.pathname.endsWith('/user/getSummaryProfile')) {
                return new Response(JSON.stringify({ success: true, body: { maSinhVien: 'student' } }));
            }
            assert.ok(target.pathname.endsWith('/dkhp/dangKyLopHocPhan'));
            const classId = target.searchParams.get('idLopHocPhan');
            const token = options.headers.get('Authorization');
            assert.equal(token, `Bearer token-user${classId}`);
            active++;
            maxActive = Math.max(maxActive, active);
            await new Promise(resolve => setTimeout(resolve, 50));
            active--;
            registered.add(classId);
            return new Response('{"success":true,"message":"Registered"}');
        };
        const users = Array.from({ length: 20 }, (_, user) => {
            const values = new Map();
            const context = {
                localStorage: { getItem: key => values.get(key) || null, setItem: (key, value) => values.set(key, value), removeItem: key => values.delete(key) },
                window: { location: {} }, AbortSignal, Response, console,
                btoa, atob, encodeURIComponent, decodeURIComponent, escape, unescape,
                fetch: async (url, options) => onRequest({ request: new Request('https://example.workers.dev' + url, options), params: { path: url.slice('/api/'.length).split('/') }, env: {} })
            };
            vm.createContext(context);
            vm.runInContext(apiSource + '\nthis.client = api;', context);
            return context.client;
        });
        const logins = await Promise.all(users.map((client, user) => client.login(`user${user}`, 'test-password')));
        assert.equal(logins.filter(result => result.success).length, 20);
        const registrations = users.map(async (client, user) => {
            await scheduledTime;
            return client.registerMainClass(String(user));
        });
        release();
        const results = await Promise.all(registrations);
        assert.equal(results.filter(result => result.success).length, 20);
        assert.equal(registered.size, 20);
        assert.ok(maxActive > 1, `expected concurrent upstream requests, got ${maxActive}`);
    } finally {
        global.fetch = originalFetch;
    }
});
