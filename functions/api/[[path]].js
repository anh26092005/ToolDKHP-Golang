export async function onRequest(context) {
    const { request, params } = context;
    const url = new URL(request.url);
    const targetBaseUrl = 'https://portal.ut.edu.vn/api/v1';
    const pathStr = Array.isArray(params.path) ? params.path.join('/') : params.path;
    const jsonResponse = (data, status = 200) => new Response(JSON.stringify(data), {
        status,
        headers: { 'Content-Type': 'application/json', 'Access-Control-Allow-Origin': '*' }
    });

    // Do not forward former key-management endpoints to the Portal.
    if (pathStr === 'keys' || pathStr.startsWith('keys/')) {
        return jsonResponse({ success: false, message: 'API không tồn tại' }, 404);
    }
    // Retain this endpoint for older cached frontend versions.
    if (pathStr === 'license/config') return jsonResponse({ required: false });

    if (pathStr === 'gateway' && request.method === 'POST') {
        let payload;
        try {
            const body = await request.json();
            const bytes = Uint8Array.from(atob(body.p), char => char.charCodeAt(0));
            payload = JSON.parse(new TextDecoder().decode(bytes));
        } catch (_) {
            return jsonResponse({ success: false, message: 'Gateway payload không hợp lệ' }, 400);
        }
        const method = String(payload.method || 'GET').toUpperCase();
        const endpoint = typeof payload.endpoint === 'string' ? payload.endpoint : '';
        if (!['GET', 'POST', 'PUT', 'DELETE'].includes(method) ||
            !/^\/(dkhp|dkhpdk|eventWeb)\//.test(endpoint) ||
            /[\\#%]/.test(endpoint.split('?')[0]) || endpoint.includes('#') ||
            endpoint.split('?')[0].split('/').some(part => part === '.' || part === '..')) {
            return jsonResponse({ success: false, message: 'Endpoint không hợp lệ' }, 400);
        }
        let target;
        try {
            target = new URL(`${targetBaseUrl}${endpoint}`);
            if (target.origin !== 'https://portal.ut.edu.vn' || !target.pathname.startsWith('/api/v1/')) throw new Error();
        } catch (_) {
            return jsonResponse({ success: false, message: 'Endpoint không hợp lệ' }, 400);
        }
        const headers = new Headers({
            'Accept': 'application/json, text/plain, */*',
            'Content-Type': 'application/json',
            'Origin': 'https://portal.ut.edu.vn',
            'Referer': 'https://portal.ut.edu.vn/'
        });
        if (payload.token) headers.set('Authorization', `Bearer ${payload.token}`);
        try {
            const options = { method, headers };
            if (method !== 'GET' && payload.data != null) options.body = JSON.stringify(payload.data);
            else if (method === 'POST') options.body = '{}';
            const response = await fetch(target.toString(), options);
            const bytes = new Uint8Array(await response.arrayBuffer());
            let binary = '';
            for (let i = 0; i < bytes.length; i += 0x8000) {
                binary += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
            }
            return jsonResponse({ r: btoa(binary) }, response.status);
        } catch (_) {
            return jsonResponse({ success: false, message: 'Không thể kết nối Portal' }, 502);
        }
    }

    if (pathStr.startsWith('registration/') || pathStr.startsWith('events/')) {
        return jsonResponse({
            success: false,
            message: 'Tính năng chạy ngầm (Server-side) chỉ hỗ trợ trên Render hoặc khi dùng bản .EXE. Trên Cloudflare, vui lòng giữ mở tab trình duyệt để hệ thống đếm ngược trực tiếp.'
        });
    }

    await new Promise(resolve => setTimeout(resolve, 800));
    const targetUrl = `${targetBaseUrl}/${pathStr}${url.search}`;
    const targetHeaders = new Headers(request.headers);
    targetHeaders.set('Origin', 'https://portal.ut.edu.vn');
    targetHeaders.set('Referer', 'https://portal.ut.edu.vn/');
    targetHeaders.set('Host', 'portal.ut.edu.vn');
    for (const name of ['cf-connecting-ip', 'cf-ipcountry', 'cf-ray', 'cf-visitor', 'x-forwarded-proto', 'x-real-ip']) targetHeaders.delete(name);
    let fetchBody = null;
    if (request.method !== 'GET' && request.method !== 'HEAD') {
        try {
            fetchBody = await request.arrayBuffer();
            if (fetchBody.byteLength === 0) fetchBody = null;
            targetHeaders.set('Content-Length', fetchBody ? fetchBody.byteLength.toString() : '0');
        } catch (_) {
            targetHeaders.set('Content-Length', '0');
        }
    }
    try {
        const response = await fetch(targetUrl, { method: request.method, headers: targetHeaders, body: fetchBody });
        const newHeaders = new Headers(response.headers);
        newHeaders.set('Access-Control-Allow-Origin', '*');
        newHeaders.set('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
        newHeaders.set('Access-Control-Allow-Headers', '*');
        return new Response(response.body, { status: response.status, statusText: response.statusText, headers: newHeaders });
    } catch (err) {
        return jsonResponse({ error: err.message }, 500);
    }
}

export async function onRequestOptions() {
    return new Response(null, {
        status: 204,
        headers: {
            'Access-Control-Allow-Origin': '*',
            'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
            'Access-Control-Allow-Headers': '*',
            'Access-Control-Max-Age': '86400',
        },
    });
}
