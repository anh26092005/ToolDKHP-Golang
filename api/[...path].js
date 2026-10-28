const PORTAL_BASE = 'https://portal.ut.edu.vn/api/v1';
const PORTAL_ORIGIN = 'https://portal.ut.edu.vn';

function json(res, status, value) {
    res.status(status).setHeader('Content-Type', 'application/json; charset=utf-8');
    res.end(JSON.stringify(value));
}

function gatewayResult(res, status, value) {
    json(res, status, { r: Buffer.from(value, 'utf8').toString('base64') });
}

function bodyObject(body) {
    if (typeof body === 'string' || Buffer.isBuffer(body)) return JSON.parse(body.toString());
    return body;
}

function portalPath(endpoint) {
    if (typeof endpoint !== 'string' || !/^\/(dkhp|dkhpdk|eventWeb)(?:\/|\?|$)/.test(endpoint)) return null;
    // Reject normalization, fragments and backslashes so the endpoint cannot escape the API prefix.
    const path = endpoint.split('?')[0];
    if (/[\\#%]/.test(path) || path.split('/').some(part => part === '.' || part === '..') || endpoint.includes('#')) return null;
    return endpoint;
}

async function proxy(res, method, endpoint, data, token, gateway) {
    const headers = {
        'Accept': 'application/json, text/plain, */*',
        'Accept-Language': 'vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7',
        'Origin': PORTAL_ORIGIN,
        'Referer': `${PORTAL_ORIGIN}/`,
        'Content-Type': 'application/json'
    };
    if (token) headers.Authorization = `Bearer ${token}`;
    try {
        const options = { method, headers, redirect: 'manual', signal: AbortSignal.timeout(15000) };
        if (data != null && method !== 'GET' && method !== 'HEAD') options.body = JSON.stringify(data);
        else if (method === 'POST') options.body = '{}';
        const upstream = await fetch(PORTAL_BASE + endpoint, options);
        const text = await upstream.text();
        // A redirect or non-JSON response is not a successful Portal API response.
        let result = text;
        try { JSON.parse(text); } catch (_) {
            result = JSON.stringify({ success: false, message: 'Phản hồi không hợp lệ từ Portal' });
        }
        const status = upstream.status >= 300 && upstream.status < 400 ? 502 : upstream.status;
        if (gateway) return gatewayResult(res, status, result);
        res.status(status).setHeader('Content-Type', 'application/json; charset=utf-8');
        return res.end(result);
    } catch (_) {
        const error = { success: false, message: 'Không thể kết nối đến Portal' };
        if (gateway) return gatewayResult(res, 502, JSON.stringify(error));
        return json(res, 502, error);
    }
}

module.exports = async function handler(req, res) {
    const url = new URL(req.url, 'http://localhost');
    const path = url.pathname.replace(/^\/api\/?/, '');
    const token = (req.headers.authorization || '').replace(/^Bearer\s+/i, '');

    if (path === 'license/config' && req.method === 'GET') return json(res, 200, { required: false });
    if (/^(registration|events)\//.test(path)) {
        return json(res, 200, { success: false, message: 'Vercel không hỗ trợ chạy tự động ngầm; vui lòng giữ mở trình duyệt để chạy trực tiếp.' });
    }
    if (path === 'gateway' && req.method === 'POST') {
        let payload;
        try {
            const body = bodyObject(req.body);
            if (!body || typeof body.p !== 'string' || !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(body.p)) throw new Error();
            payload = JSON.parse(Buffer.from(body.p, 'base64').toString('utf8'));
        } catch (_) {
            return json(res, 400, { success: false, message: 'Gateway payload không hợp lệ' });
        }
        const endpoint = portalPath(payload && payload.endpoint);
        const method = payload && payload.method;
        if (!endpoint || !['GET', 'POST', 'PUT', 'DELETE'].includes(method) || (payload.token != null && typeof payload.token !== 'string')) {
            return json(res, 400, { success: false, message: 'Gateway endpoint hoặc method không hợp lệ' });
        }
        return proxy(res, method, endpoint, payload.data, payload.token || token, true);
    }
    if (path === 'user/login' && req.method === 'POST') {
        let body;
        try { body = bodyObject(req.body); } catch (_) { /* invalid JSON */ }
        if (!body || typeof body.username !== 'string' || !body.username || typeof body.password !== 'string' || !body.password) {
            return json(res, 400, { success: false, message: 'Vui lòng nhập tên đăng nhập và mật khẩu' });
        }
        return proxy(res, 'POST', '/user/login', { username: body.username, password: body.password }, '', false);
    }
    if (path === 'user/getSummaryProfile' && req.method === 'GET') {
        if (!/^Bearer\s+\S+/i.test(req.headers.authorization || '')) return json(res, 401, { success: false, message: 'Chưa đăng nhập' });
        return proxy(res, 'GET', '/user/getSummaryProfile', null, token, false);
    }
    return json(res, 404, { success: false, message: 'API không tồn tại' });
};
