export async function onRequest(context) {
    const { request, params, env } = context;
    const url = new URL(request.url);

    // Target API URL (School Portal)
    const targetBaseUrl = 'https://portal.ut.edu.vn/api/v1';

    // Construct the target URL
    const path = params.path;
    const pathStr = Array.isArray(path) ? path.join('/') : path;

    // ==================== KV & Admin Logic ====================

    // Set ADMIN_PASSWORD as a Cloudflare Pages secret.
    const ADMIN_PASSWORD = env.ADMIN_PASSWORD;

    // Helper: Response JSON
    const jsonResponse = (data, status = 200) => new Response(JSON.stringify(data), {
        status,
        headers: { 'Content-Type': 'application/json', 'Access-Control-Allow-Origin': '*' }
    });

    if (pathStr && pathStr.startsWith('keys/admin/') && !ADMIN_PASSWORD) {
        return jsonResponse({ success: false, message: 'Chưa cấu hình ADMIN_PASSWORD trên Cloudflare' }, 503);
    }

    if (pathStr === 'license/config') {
        return jsonResponse({ required: false });
    }

    // The browser sends Portal calls as a base64-wrapped gateway payload.
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

    // 1. Validate Key Endpoint
    if (pathStr === 'keys/validate') {
        let body = {};
        try { body = await request.json(); } catch (e) { }
        const key = (body.key || '').trim().toUpperCase();
        const deviceId = body.deviceId;

        if (!key) return jsonResponse({ allowed: false, message: 'Vui lòng nhập key' });

        // Check KV
        const keyDataStr = await env.DKHP_KEYS.get(key);
        if (!keyDataStr) {
            return jsonResponse({ allowed: false, message: 'Key không tồn tại' });
        }

        const keyData = JSON.parse(keyDataStr);

        // Calculate expiresAt if it's the first time and duration is set
        const calculateExpiration = (data) => {
            if (!data.expiresAt && data.duration) {
                const now = new Date();
                if (data.duration === '1day') return new Date(now.getTime() + 1 * 86400000).toISOString();
                if (data.duration === '7days') return new Date(now.getTime() + 7 * 86400000).toISOString();
                if (data.duration === '30days') return new Date(now.getTime() + 30 * 86400000).toISOString();
            }
            return data.expiresAt;
        };

        // Shared key: Allow & Update stats
        if (keyData.keyType === 'shared') {
            if (!keyData.boundAt) {
                keyData.boundAt = new Date().toISOString();
                keyData.expiresAt = calculateExpiration(keyData);
            }
            
            // Check expiration
            if (keyData.expiresAt && new Date(keyData.expiresAt) < new Date()) {
                return jsonResponse({ allowed: false, message: 'Key đã hết hạn' });
            }

            keyData.usageCount = (keyData.usageCount || 0) + 1;
            keyData.lastUsed = new Date().toISOString();
            // Update KV (background)
            context.waitUntil(env.DKHP_KEYS.put(key, JSON.stringify(keyData)));
            return jsonResponse({ allowed: true, keyType: 'shared' });
        }

        // Check expiration for regular keys (if already bound and expired)
        if (keyData.expiresAt && new Date(keyData.expiresAt) < new Date()) {
            return jsonResponse({ allowed: false, message: 'Key đã hết hạn' });
        }

        // Regular key: Check Device Binding
        if (!deviceId) return jsonResponse({ allowed: false, message: 'Thiếu Device ID' });

        if (!keyData.deviceId) {
            // First bind: Set device and calculate expiration from now
            keyData.deviceId = deviceId;
            keyData.boundAt = new Date().toISOString();
            keyData.expiresAt = calculateExpiration(keyData);
            keyData.usageCount = 1;
            keyData.lastUsed = new Date().toISOString();
            await env.DKHP_KEYS.put(key, JSON.stringify(keyData));
            return jsonResponse({ allowed: true, keyType: 'regular', newBinding: true });
        } else if (keyData.deviceId !== deviceId) {
            return jsonResponse({ allowed: false, message: 'Key đã dùng trên thiết bị khác' });
        } else {
            // Valid device
            keyData.usageCount = (keyData.usageCount || 0) + 1;
            keyData.lastUsed = new Date().toISOString();
            context.waitUntil(env.DKHP_KEYS.put(key, JSON.stringify(keyData)));
            return jsonResponse({ allowed: true, keyType: 'regular' });
        }
    }

    // 2. Admin Login
    if (pathStr === 'keys/admin/login') {
        let body = {};
        try { body = await request.json(); } catch (e) { }
        if (body.password === ADMIN_PASSWORD) {
            return jsonResponse({ success: true, message: 'Login OK' });
        }
        return jsonResponse({ success: false, message: 'Sai mật khẩu' }, 401);
    }

    // 3. Admin: Generate Key
    if (pathStr === 'keys/admin/keys/generate') {
        // Auth Check
        const auth = request.headers.get('Authorization');
        if (auth !== `Bearer ${ADMIN_PASSWORD}`) return jsonResponse({ success: false }, 401);

        let body = {};
        try { body = await request.json(); } catch (e) { }
        const { keyType = 'regular', duration = '7days', customKey } = body;

        let newKey;
        if (customKey && customKey.trim() !== '') {
            newKey = customKey.trim().toUpperCase();
            // Check if key already exists
            const existing = await env.DKHP_KEYS.get(newKey);
            if (existing) {
                return jsonResponse({ success: false, message: 'Key này đã tồn tại, vui lòng chọn tên khác' });
            }
        } else {
            const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
            let attempts = 0;
            do {
                newKey = 'DKHP-';
                for (let i = 0; i < 4; i++) newKey += chars.charAt(Math.floor(Math.random() * chars.length));
                newKey += '-';
                for (let i = 0; i < 4; i++) newKey += chars.charAt(Math.floor(Math.random() * chars.length));
                
                const existing = await env.DKHP_KEYS.get(newKey);
                if (!existing) break;
                attempts++;
            } while (attempts < 10);

            if (attempts >= 10) {
                return jsonResponse({ success: false, message: 'Không thể tạo key duy nhất' });
            }
        }

        const now = new Date();
        const keyData = {
            key: newKey,
            keyType,
            createdAt: now.toISOString(),
            duration,
            expiresAt: null, // Calculated on first activation
            deviceId: null,
            usageCount: 0
        };

        await env.DKHP_KEYS.put(newKey, JSON.stringify(keyData));
        return jsonResponse({ success: true, key: keyData, message: `Đã tạo key: ${newKey}` });
    }

    // 4. Admin: List Keys
    if (pathStr === 'keys/admin/keys' && request.method === 'GET') {
        const auth = request.headers.get('Authorization');
        if (auth !== `Bearer ${ADMIN_PASSWORD}`) return jsonResponse({ success: false }, 401);

        const list = await env.DKHP_KEYS.list();
        const keys = [];
        for (const key of list.keys) {
            const val = await env.DKHP_KEYS.get(key.name);
            if (val) keys.push(JSON.parse(val));
        }
        // Sort newest
        keys.sort((a, b) => new Date(b.createdAt) - new Date(a.createdAt));
        return jsonResponse({ success: true, keys });
    }

    // 5. Admin: Delete Key
    if (pathStr.startsWith('keys/admin/keys/') && request.method === 'DELETE') {
        const auth = request.headers.get('Authorization');
        if (auth !== `Bearer ${ADMIN_PASSWORD}`) return jsonResponse({ success: false }, 401);

        const keyToDelete = pathStr.split('/').pop(); // Get last segment
        await env.DKHP_KEYS.delete(keyToDelete);
        return jsonResponse({ success: true, message: 'Đã xóa key' });
    }

    // 6. Admin: Reset Key
    if (pathStr.includes('/reset') && request.method === 'POST') {
        const auth = request.headers.get('Authorization');
        if (auth !== `Bearer ${ADMIN_PASSWORD}`) return jsonResponse({ success: false }, 401);

        // Extract key from path: keys/admin/keys/DKHP-XXXX-XXXX/reset
        const parts = pathStr.split('/');
        const keyToReset = parts[parts.length - 2]; // 2nd to last

        const val = await env.DKHP_KEYS.get(keyToReset);
        if (val) {
            const data = JSON.parse(val);
            data.deviceId = null;
            data.boundAt = null;
            await env.DKHP_KEYS.put(keyToReset, JSON.stringify(data));
            return jsonResponse({ success: true, message: 'Đã reset binding' });
        }
        return jsonResponse({ success: false, message: 'Key not found' });
    }

    // 7. Admin: Stats
    if (pathStr === 'keys/admin/stats') {
        const auth = request.headers.get('Authorization');
        if (auth !== `Bearer ${ADMIN_PASSWORD}`) return jsonResponse({ success: false }, 401);

        const list = await env.DKHP_KEYS.list();
        let total = 0, activeRegular = 0, activeShared = 0, boundKeys = 0, usedToday = 0;
        const today = new Date().toISOString().split('T')[0];

        for (const k of list.keys) {
            const val = await env.DKHP_KEYS.get(k.name);
            if (val) {
                const d = JSON.parse(val);
                total++;
                if (d.keyType === 'shared') activeShared++; else activeRegular++;
                if (d.deviceId) boundKeys++;
                if (d.lastUsed && d.lastUsed.startsWith(today)) usedToday++;
            }
        }
        return jsonResponse({ success: true, stats: { total, activeRegular, activeShared, boundKeys, usedToday } });
    }


    // ==================== Intercept Node.js only routes ====================
    if (pathStr.startsWith('registration/') || pathStr.startsWith('events/')) {
        return jsonResponse({ 
            success: false, 
            message: 'Tính năng chạy ngầm (Server-side) chỉ hỗ trợ trên Render hoặc khi dùng bản .EXE. Trên Cloudflare, vui lòng giữ mở tab trình duyệt để hệ thống đếm ngược trực tiếp.' 
        });
    }

    // ==================== Proxy Logic ====================

    // Add 0.8s delay to avoid detection (Throttling)
    await new Promise(resolve => setTimeout(resolve, 800));

    const targetUrl = `${targetBaseUrl}/${pathStr}${url.search}`;

    // Prepare headers for the target request
    const targetHeaders = new Headers(request.headers);
    targetHeaders.set('Origin', 'https://portal.ut.edu.vn');
    targetHeaders.set('Referer', 'https://portal.ut.edu.vn/');
    targetHeaders.set('Host', 'portal.ut.edu.vn');

    // Remove headers that might cause issues
    targetHeaders.delete('cf-connecting-ip');
    targetHeaders.delete('cf-ipcountry');
    targetHeaders.delete('cf-ray');
    targetHeaders.delete('cf-visitor');
    targetHeaders.delete('x-forwarded-proto');
    targetHeaders.delete('x-real-ip');

    let fetchBody = null;
    if (request.method !== 'GET' && request.method !== 'HEAD') {
        try {
            fetchBody = await request.arrayBuffer();
            if (fetchBody.byteLength === 0) {
                fetchBody = null;
                targetHeaders.set('Content-Length', '0');
            } else {
                targetHeaders.set('Content-Length', fetchBody.byteLength.toString());
            }
        } catch (e) {
            // Ignore if body can't be read
            fetchBody = null;
            targetHeaders.set('Content-Length', '0');
        }
    }

    try {
        const response = await fetch(targetUrl, {
            method: request.method,
            headers: targetHeaders,
            body: fetchBody,
        });

        // Create a new response with CORS headers allowed
        const newHeaders = new Headers(response.headers);
        newHeaders.set('Access-Control-Allow-Origin', '*');
        newHeaders.set('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
        newHeaders.set('Access-Control-Allow-Headers', '*');

        return new Response(response.body, {
            status: response.status,
            statusText: response.statusText,
            headers: newHeaders,
        });
    } catch (err) {
        return new Response(JSON.stringify({ error: err.message }), {
            status: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            }
        });
    }
}

// Handle OPTIONS requests for CORS preflight
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
