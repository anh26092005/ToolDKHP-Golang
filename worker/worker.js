/**
 * ĐKHP Web - Cloudflare Worker
 * Key-Based Authentication API
 * 2 loại key: regular (1 user) | shared (unlimited users)
 */

const ADMIN_PASSWORD = 'Concat123@'; // Đổi mật khẩu này!

// CORS headers
const corsHeaders = {
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Methods': 'GET, POST, DELETE, OPTIONS',
    'Access-Control-Allow-Headers': 'Content-Type, Authorization',
    'Content-Type': 'application/json'
};

// Handle OPTIONS (CORS preflight)
function handleOptions() {
    return new Response(null, { headers: corsHeaders });
}

// Parse JSON body
async function parseBody(request) {
    try {
        return await request.json();
    } catch {
        return {};
    }
}

// Check admin auth
function checkAdmin(request) {
    const auth = request.headers.get('Authorization');
    return auth === `Bearer ${ADMIN_PASSWORD}`;
}

// JSON response helper
function jsonResponse(data, status = 200) {
    return new Response(JSON.stringify(data), {
        status,
        headers: corsHeaders
    });
}

// Generate random key: DKHP-XXXX-XXXX
function generateKey() {
    const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789'; // No I,O,0,1 to avoid confusion
    let key = 'DKHP-';
    for (let i = 0; i < 4; i++) {
        key += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    key += '-';
    for (let i = 0; i < 4; i++) {
        key += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return key;
}

// Calculate expiration date
function getExpirationDate(duration) {
    const now = new Date();
    switch (duration) {
        case '1day': return new Date(now.getTime() + 1 * 24 * 60 * 60 * 1000).toISOString();
        case '7days': return new Date(now.getTime() + 7 * 24 * 60 * 60 * 1000).toISOString();
        case '30days': return new Date(now.getTime() + 30 * 24 * 60 * 60 * 1000).toISOString();
        case 'forever': return null; // null = never expires
        default: return new Date(now.getTime() + 7 * 24 * 60 * 60 * 1000).toISOString(); // Default 7 days
    }
}

// ==================== Main Handler ====================
export default {
    async fetch(request, env, ctx) {
        const url = new URL(request.url);
        const path = url.pathname;
        const method = request.method;

        // Handle CORS preflight
        if (method === 'OPTIONS') {
            return handleOptions();
        }

        // ==================== Admin Routes ====================

        // Admin login
        if (path === '/admin/login' && method === 'POST') {
            const { password } = await parseBody(request);
            if (password === ADMIN_PASSWORD) {
                return jsonResponse({ success: true, message: 'Đăng nhập thành công' });
            }
            return jsonResponse({ success: false, message: 'Mật khẩu sai' }, 401);
        }

        // Generate new key
        if (path === '/admin/keys/generate' && method === 'POST') {
            if (!checkAdmin(request)) {
                return jsonResponse({ success: false, message: 'Unauthorized' }, 401);
            }

            const { keyType = 'regular', duration = '7days' } = await parseBody(request);

            // Generate unique key
            let key;
            let attempts = 0;
            do {
                key = generateKey();
                const existing = await env.KEYS.get(key);
                if (!existing) break;
                attempts++;
            } while (attempts < 10);

            if (attempts >= 10) {
                return jsonResponse({ success: false, message: 'Không thể tạo key duy nhất' });
            }

            const keyData = {
                key,
                keyType, // 'regular' or 'shared'
                createdAt: new Date().toISOString(),
                expiresAt: getExpirationDate(duration),
                deviceId: null,
                boundAt: null,
                lastUsed: null,
                usageCount: 0
            };

            await env.KEYS.put(key, JSON.stringify(keyData));

            return jsonResponse({
                success: true,
                message: `Đã tạo ${keyType === 'shared' ? 'key chung' : 'key thường'}: ${key}`,
                key: keyData
            });
        }

        // Get all keys
        if (path === '/admin/keys' && method === 'GET') {
            if (!checkAdmin(request)) {
                return jsonResponse({ success: false, message: 'Unauthorized' }, 401);
            }

            const list = [];
            const keys = await env.KEYS.list();

            for (const keyObj of keys.keys) {
                const data = await env.KEYS.get(keyObj.name, 'json');
                if (data) list.push(data);
            }

            // Sort newest first
            list.sort((a, b) => new Date(b.createdAt) - new Date(a.createdAt));

            return jsonResponse({ success: true, keys: list });
        }

        // Delete key
        if (path.startsWith('/admin/keys/') && method === 'DELETE') {
            if (!checkAdmin(request)) {
                return jsonResponse({ success: false, message: 'Unauthorized' }, 401);
            }

            // Extract key from path: /admin/keys/DKHP-XXXX-XXXX
            const key = decodeURIComponent(path.replace('/admin/keys/', ''));
            const existing = await env.KEYS.get(key);

            if (!existing) {
                return jsonResponse({ success: false, message: 'Key không tồn tại' }, 404);
            }

            await env.KEYS.delete(key);
            return jsonResponse({ success: true, message: 'Đã xóa key' });
        }

        // Reset device binding (only for regular keys)
        if (path.endsWith('/reset') && method === 'POST') {
            if (!checkAdmin(request)) {
                return jsonResponse({ success: false, message: 'Unauthorized' }, 401);
            }

            // Path: /admin/keys/DKHP-XXXX-XXXX/reset
            const pathParts = path.split('/');
            const key = decodeURIComponent(pathParts[pathParts.length - 2]);

            const keyData = await env.KEYS.get(key, 'json');

            if (!keyData) {
                return jsonResponse({ success: false, message: 'Key không tồn tại' }, 404);
            }

            if (keyData.keyType === 'shared') {
                return jsonResponse({ success: false, message: 'Key chung không cần reset' });
            }

            // Reset device binding
            keyData.deviceId = null;
            keyData.boundAt = null;
            await env.KEYS.put(key, JSON.stringify(keyData));

            return jsonResponse({ success: true, message: 'Đã reset device binding' });
        }

        // Stats
        if (path === '/admin/stats' && method === 'GET') {
            if (!checkAdmin(request)) {
                return jsonResponse({ success: false, message: 'Unauthorized' }, 401);
            }

            const keys = await env.KEYS.list();
            const now = new Date();
            const today = now.toISOString().split('T')[0];

            let total = 0;
            let activeRegular = 0;
            let activeShared = 0;
            let boundKeys = 0;
            let expiredKeys = 0;
            let usedToday = 0;

            for (const keyObj of keys.keys) {
                const data = await env.KEYS.get(keyObj.name, 'json');
                if (!data) continue;

                total++;

                // Check expired
                if (data.expiresAt && new Date(data.expiresAt) < now) {
                    expiredKeys++;
                } else {
                    if (data.keyType === 'shared') activeShared++;
                    else activeRegular++;
                }

                if (data.deviceId) boundKeys++;
                if (data.lastUsed && data.lastUsed.startsWith(today)) usedToday++;
            }

            return jsonResponse({
                success: true,
                stats: {
                    total,
                    activeRegular,
                    activeShared,
                    boundKeys,
                    expiredKeys,
                    usedToday
                }
            });
        }

        // ==================== User Routes ====================

        // Validate key
        if (path === '/validate' && method === 'POST') {
            const { key, deviceId } = await parseBody(request);

            if (!key || !key.trim()) {
                return jsonResponse({ allowed: false, message: 'Vui lòng nhập key' });
            }

            const keyClean = key.trim().toUpperCase();
            const keyData = await env.KEYS.get(keyClean, 'json');

            if (!keyData) {
                return jsonResponse({
                    allowed: false,
                    message: 'Key không hợp lệ'
                });
            }

            // Check expiration
            if (keyData.expiresAt && new Date(keyData.expiresAt) < new Date()) {
                return jsonResponse({
                    allowed: false,
                    message: 'Key đã hết hạn. Vui lòng liên hệ admin để gia hạn!'
                });
            }

            // For shared keys: always allow
            if (keyData.keyType === 'shared') {
                keyData.lastUsed = new Date().toISOString();
                keyData.usageCount = (keyData.usageCount || 0) + 1;
                await env.KEYS.put(keyClean, JSON.stringify(keyData));

                return jsonResponse({ allowed: true, keyType: 'shared' });
            }

            // For regular keys: check device binding
            if (!deviceId) {
                return jsonResponse({ allowed: false, message: 'Thiếu thông tin thiết bị' });
            }

            // Not bound yet - bind now
            if (!keyData.deviceId) {
                keyData.deviceId = deviceId;
                keyData.boundAt = new Date().toISOString();
                keyData.lastUsed = new Date().toISOString();
                keyData.usageCount = (keyData.usageCount || 0) + 1;
                await env.KEYS.put(keyClean, JSON.stringify(keyData));

                return jsonResponse({ allowed: true, keyType: 'regular', newBinding: true });
            }

            // Already bound - check if same device
            if (keyData.deviceId !== deviceId) {
                return jsonResponse({
                    allowed: false,
                    message: 'Key đã được sử dụng trên thiết bị khác. Vui lòng liên hệ admin để biết thêm chi tiết!'
                });
            }

            // Same device - allow
            keyData.lastUsed = new Date().toISOString();
            keyData.usageCount = (keyData.usageCount || 0) + 1;
            await env.KEYS.put(keyClean, JSON.stringify(keyData));

            return jsonResponse({ allowed: true, keyType: 'regular' });
        }

        // Backward compatibility: /check endpoint (deprecated)
        if (path === '/check' && method === 'POST') {
            return jsonResponse({
                allowed: false,
                message: 'API đã thay đổi. Vui lòng nhập key thay vì MSSV!'
            });
        }

        // 404
        return jsonResponse({ error: 'Not found' }, 404);
    }
};
