const { test, afterEach } = require('node:test');
const assert = require('node:assert/strict');
const handler = require('../api/[...path].js');

const originalFetch = global.fetch;
afterEach(() => { global.fetch = originalFetch; });

async function request(path, method = 'GET', body, authorization) {
    const res = {
        status(code) { this.statusCode = code; return this; },
        setHeader(name, value) { (this.headers ||= {})[name] = value; return this; },
        end(value) { this.body = JSON.parse(value); }
    };
    await handler({ url: path, method, body, headers: authorization ? { authorization } : {} }, res);
    return res;
}

test('static output and fallback configuration', async () => {
    const config = require('../vercel.json');
    assert.equal(config.outputDirectory, 'public');
    assert.equal(require('node:fs').existsSync('public/index.html'), true);
    assert.deepEqual((await request('/api/license/config')).body, { required: false });
    for (const path of ['/api/registration/auto/start', '/api/events/auto/start']) {
        const res = await request(path, 'POST');
        assert.equal(res.body.success, false);
        assert.match(res.body.message, /trình duyệt/);
    }
});

test('login and profile forward credentials and bearer token to Portal', async () => {
    const calls = [];
    global.fetch = async (url, options) => {
        calls.push({ url, options });
        return new Response(JSON.stringify(url.endsWith('/user/login')
            ? { success: true, token: 'abc' }
            : { success: true, body: { hoTen: 'Nguyễn An' } }));
    };
    const login = await request('/api/user/login', 'POST', { username: 'sv', password: 'pw' });
    assert.equal(login.body.token, 'abc');
    assert.equal(calls[0].url, 'https://portal.ut.edu.vn/api/v1/user/login');
    assert.deepEqual(JSON.parse(calls[0].options.body), { username: 'sv', password: 'pw' });
    const profile = await request('/api/user/getSummaryProfile', 'GET', null, 'Bearer abc');
    assert.equal(profile.body.body.hoTen, 'Nguyễn An');
    assert.equal(calls[1].options.headers.Authorization, 'Bearer abc');
    assert.equal((await request('/api/user/getSummaryProfile')).statusCode, 401);
});

test('gateway preserves method, query, body, token, UTF-8 and status', async () => {
    let call;
    global.fetch = async (url, options) => {
        call = { url, options };
        return new Response(JSON.stringify({ success: false, message: 'Lớp đã đầy' }), { status: 409 });
    };
    const payload = { method: 'POST', endpoint: '/dkhp/dangKyLopHocPhan?idLopHocPhan=123', data: { ghiChu: 'Đăng ký' }, token: 'payload-token' };
    const res = await request('/api/gateway', 'POST', { p: Buffer.from(JSON.stringify(payload)).toString('base64') }, 'Bearer header-token');
    assert.equal(res.statusCode, 409);
    assert.equal(call.url, 'https://portal.ut.edu.vn/api/v1/dkhp/dangKyLopHocPhan?idLopHocPhan=123');
    assert.equal(call.options.method, 'POST');
    assert.equal(call.options.headers.Authorization, 'Bearer payload-token');
    assert.deepEqual(JSON.parse(call.options.body), payload.data);
    assert.equal(JSON.parse(Buffer.from(res.body.r, 'base64').toString()).message, 'Lớp đã đầy');
});

test('rejects malformed or escaping gateway requests and reports Portal failures', async () => {
    let called = false;
    global.fetch = async () => { called = true; throw new Error('offline'); };
    assert.equal((await request('/api/gateway', 'POST', { p: 'bad!' })).statusCode, 400);
    for (const endpoint of ['/dkhp/../user/login', '/dkhp/%2e%2e/user/login', '//evil.test/x']) {
        const p = Buffer.from(JSON.stringify({ method: 'GET', endpoint })).toString('base64');
        assert.equal((await request('/api/gateway', 'POST', { p })).statusCode, 400);
    }
    assert.equal(called, false);
    const p = Buffer.from(JSON.stringify({ method: 'GET', endpoint: '/eventWeb/list' })).toString('base64');
    const res = await request('/api/gateway', 'POST', { p });
    assert.equal(res.statusCode, 502);
    assert.equal(JSON.parse(Buffer.from(res.body.r, 'base64').toString()).success, false);
});
