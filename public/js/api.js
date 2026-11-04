/**
 * ĐKHP Web - API Client
 */

class ApiClient {
    constructor() {
        // Always use the proxy path. 
        // On Localhost: server.js handles /api/...
        // On Cloudflare: functions/api/[[path]].js handles /api/...
        this.baseUrl = '/api';

        // Token storage
        this.token = localStorage.getItem('dkhp_token') || null;
        this.studentInfo = JSON.parse(localStorage.getItem('dkhp_student') || 'null');
        this._reloginPromise = null;
        this._reloginError = null;
        this._needsRelogin = false;
        // The Go upstream attempt is bounded at 15s; allow it time to return its timeout response.
        this.requestTimeoutMs = 20000;

        // Auto register state
        this.autoRegisterRunning = false;
        this.autoRegisterStop = false;
        this.logs = []; // Store logs for polling
    }

    // ==================== Credentials & Auto Re-Login ====================

    saveCredentials(username, password) {
        try {
            const data = JSON.stringify({ username, password, savedAt: Date.now() });
            const encoded = btoa(encodeURIComponent(data));
            localStorage.setItem('dkhp_credentials', encoded);
        } catch (e) {
            console.error('Không thể lưu credentials:', e);
        }
    }

    getCredentials() {
        try {
            const encoded = localStorage.getItem('dkhp_credentials');
            if (!encoded) return null;
            const decoded = decodeURIComponent(atob(encoded));
            return JSON.parse(decoded);
        } catch (e) {
            return null;
        }
    }

    clearCredentials() {
        localStorage.removeItem('dkhp_credentials');
    }

    async tryAutoRelogin(shouldContinue = null, onProgress = null) {
        if (shouldContinue && !shouldContinue()) return false;
        if (onProgress) onProgress('Đang đăng nhập lại — mỗi yêu cầu tối đa 20 giây...');
        if (this._reloginPromise) {
            return await this._reloginPromise;
        }

        const creds = this.getCredentials();
        if (!creds || !creds.username || !creds.password) {
            this._reloginError = { success: false, isUnauthorized: true, isAuthenticationFailed: true,
                message: 'Không có thông tin đăng nhập đã lưu. Vui lòng đăng nhập và bật Ghi nhớ.' };
            return false;
        }

        this._reloginPromise = (async () => {
            try {
                const response = await fetch(`${this.baseUrl}/user/login`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'include',
                    signal: AbortSignal.timeout(this.requestTimeoutMs),
                    body: JSON.stringify({
                        username: creds.username,
                        password: creds.password
                    })
                });

                let data;
                try { data = await response.json(); } catch (_) { /* HTML/empty replies during an outage are temporary. */ }
                if (response.ok && data?.success === true && typeof data.token === 'string' && data.token) {
                    this.token = data.token;
                    this._needsRelogin = false;
                    this._reloginError = null;
                    localStorage.setItem('dkhp_token', this.token);
                    if (data.studentInfo) {
                        this.studentInfo = data.studentInfo;
                        localStorage.setItem('dkhp_student', JSON.stringify(this.studentInfo));
                    }
                    console.log('[AutoReLogin] ✅ Tự động đăng nhập lại thành công! Đã cấp token mới.');
                    return true;
                }
                const rejected = [400, 401, 403].includes(response.status) || (response.ok && data?.success === false &&
                    /(?:sai|không (?:chính xác|đúng|hợp lệ)|incorrect|invalid|wrong).*(?:mật khẩu|password|tài khoản)|(?:mật khẩu|password|tài khoản).*(?:sai|không (?:chính xác|đúng|hợp lệ)|incorrect|invalid|wrong)/i.test(data.message || ''));
                this._reloginError = { success: false, isUnauthorized: true, statusCode: response.status,
                    isAuthenticationFailed: rejected, isServerError: !rejected,
                    message: rejected ? 'Portal từ chối tài khoản/mật khẩu. Vui lòng kiểm tra và đăng nhập lại.'
                        : 'Portal chưa đăng nhập lại được. Tiến trình sẽ chờ và thử lại.' };
                return false;
            } catch (err) {
                this._reloginError = { success: false, isUnauthorized: true, isNetworkError: true,
                    message: 'Đăng nhập lại bị timeout/lỗi kết nối. Tiến trình sẽ thử lại khi Portal hồi phục.' };
                return false;
            } finally {
                this._reloginPromise = null;
            }
        })();

        return await this._reloginPromise;
    }

    // ==================== Safe Base64 Helper ====================
    _encodePayload(obj) {
        try {
            return btoa(unescape(encodeURIComponent(JSON.stringify(obj))));
        } catch (e) {
            return btoa(JSON.stringify(obj));
        }
    }

    _decodePayload(str) {
        try {
            return JSON.parse(decodeURIComponent(escape(atob(str))));
        } catch (e) {
            return JSON.parse(atob(str));
        }
    }

    // ==================== Unified Request Method ====================

    isExpiredSession(result) {
        if (!result || result.success === true) return false;
        return result.isUnauthorized === true || /unauthorized|hết phiên|phiên làm việc hết hạn|token.*(?:hết hạn|expired|invalid)/i.test(result.message || '');
    }

    async recoverUnauthorized(method, endpoint, data, isRetry, shouldContinue, onProgress, sentToken = null) {
        // Another request may already have refreshed this old token while its 401 was in flight.
        if (!isRetry && sentToken && this.token && sentToken !== this.token) {
            return this.request(method, endpoint, data, true, shouldContinue, onProgress);
        }
        this._needsRelogin = true;
        if (!isRetry && await this.tryAutoRelogin(shouldContinue, onProgress)) {
            if (shouldContinue && !shouldContinue()) return { success: false, isCancelled: true, message: 'Đã dừng yêu cầu' };
            if (onProgress) onProgress('Đã đăng nhập lại; đang gửi lại yêu cầu với token mới...');
            return this.request(method, endpoint, data, true, shouldContinue, onProgress);
        }
        return { success: false, isUnauthorized: true, statusCode: 401,
            message: 'Phiên đã hết hạn; đang chờ đăng nhập lại.', ...this._reloginError };
    }

    async request(method, endpoint, data = null, isRetry = false, shouldContinue = null, onProgress = null) {
        if (shouldContinue && !shouldContinue()) {
            return { success: false, isCancelled: true, message: 'Đã dừng yêu cầu' };
        }
        // Obfuscate and route portal endpoints through /gateway
        const isPortalEndpoint = endpoint.startsWith('/dkhp') || endpoint.startsWith('/dkhpdk') || endpoint.startsWith('/eventWeb');

        // After a failed refresh, retry login first instead of repeatedly sending a known-expired token.
        if (this._needsRelogin && !isRetry && endpoint !== '/user/login') {
            return this.recoverUnauthorized(method, endpoint, data, false, shouldContinue, onProgress);
        }

        let targetUrl = `${this.baseUrl}${endpoint}`;
        let requestMethod = method;
        let requestBody = null;
        const sentToken = this.token;

        const headers = { 'Content-Type': 'application/json' };
        if (this.token) {
            headers['Authorization'] = `Bearer ${this.token}`;
        }

        if (isPortalEndpoint) {
            targetUrl = `${this.baseUrl}/gateway`;
            requestMethod = 'POST';
            const payload = {
                method,
                endpoint,
                data,
                token: this.token
            };
            requestBody = JSON.stringify({ p: this._encodePayload(payload) });
        } else if (data && (method === 'POST' || method === 'PUT')) {
            requestBody = JSON.stringify(data);
        }

        const options = {
            method: requestMethod,
            headers,
            credentials: 'include',
            signal: AbortSignal.timeout(this.requestTimeoutMs)
        };
        if (requestBody) {
            options.body = requestBody;
        }

        try {
            const response = await fetch(targetUrl, options);

            // Handle 401 (Unauthorized)
            if (response.status === 401) {
                if (endpoint !== '/user/login') {
                    if (response.body) await response.body.cancel();
                    return this.recoverUnauthorized(method, endpoint, data, isRetry, shouldContinue, onProgress, sentToken);
                }
                let msg = 'Tài khoản hoặc mật khẩu không chính xác';
                try {
                    const errorText = await response.text();
                    const parsed = JSON.parse(errorText);
                    if (parsed && parsed.message) msg = parsed.message;
                } catch (e) { }
                return { success: false, message: msg, isUnauthorized: true, statusCode: 401 };
            }

            // Handle 429 (Rate Limit)
            if (response.status === 429) {
                if (response.body) await response.body.cancel();
                const retryAfter = response.headers.get('Retry-After');
                const retryAfterMs = /^\d+$/.test(retryAfter || '') ? Number(retryAfter) * 1000 : Date.parse(retryAfter) - Date.now();
                return { success: false, message: 'Portal đang giới hạn tần suất gửi', isRateLimit: true, statusCode: 429,
                    retryAfterMs: Number.isFinite(retryAfterMs) ? Math.min(Math.max(retryAfterMs, 0), 300000) : 0 };
            }

            // Handle 500, 502, 503, 504 (Server quá tải hoặc lỗi cổng trường)
            if (response.status >= 500 || response.status === 408) {
                let errorMsg = `Cổng trường quá tải/phản hồi lỗi (${response.status})`;
                try {
                    const errorText = await response.text();
                    const parsed = JSON.parse(errorText);
                    if (parsed.r) {
                        const decoded = this._decodePayload(parsed.r);
                        if (decoded && decoded.message) errorMsg = decoded.message;
                    } else if (parsed.message) {
                        errorMsg = parsed.message;
                    }
                } catch (e) { }
                return {
                    success: false,
                    message: errorMsg,
                    isServerError: true,
                    statusCode: response.status
                };
            }

            const text = await response.text();
            if (!text) {
                if (endpoint.startsWith('/eventWeb/')) {
                    return { success: false, message: 'Portal chưa xác nhận kết quả (phản hồi trống)' };
                }
                if (response.ok) {
                    return { success: true, message: 'Thành công (Không có dữ liệu trả về)' };
                } else {
                    return { success: false, message: `Lỗi từ server (Status: ${response.status})` };
                }
            }

            try {
                let result = JSON.parse(text);
                // If response is encapsulated in obfuscated gateway response { r: "..." }
                if (result && typeof result.r === 'string') {
                    result = this._decodePayload(result.r);
                }
                if (endpoint !== '/user/login' && this.isExpiredSession(result)) {
                    return this.recoverUnauthorized(method, endpoint, data, isRetry, shouldContinue, onProgress, sentToken);
                }
                if (!response.ok) return { ...result, success: false, statusCode: response.status };
                return result;
            } catch (err) {
                return {
                    success: false,
                    message: `Lỗi dữ liệu từ server (Status ${response.status})`,
                    isServerError: true
                };
            }
        } catch (error) {
            return {
                success: false,
                message: `Lỗi kết nối mạng: ${error.message}`,
                isNetworkError: true
            };
        }
    }

    // ==================== Auth ====================

    async login(username, password, remember = true) {
        // Lưu credentials nếu bật remember
        if (remember) {
            this.saveCredentials(username, password);
        } else {
            this.clearCredentials();
        }

        // Login to Portal (via Proxy)
        const loginResult = await this.request('POST', '/user/login', { username, password });

        if (loginResult.success && loginResult.token) {
            this.token = loginResult.token;
            this._needsRelogin = false;
            this._reloginError = null;
            localStorage.setItem('dkhp_token', this.token);

            // Go returns a usable identity with the token; avoid waiting on an optional profile during peak load.
            if (loginResult.studentInfo) {
                this.studentInfo = loginResult.studentInfo;
                localStorage.setItem('dkhp_student', JSON.stringify(this.studentInfo));
                return { success: true, message: 'Đăng nhập thành công', studentInfo: this.studentInfo };
            }

            // Get profile
            const profileResult = await this.request('GET', '/user/getSummaryProfile');
            if (profileResult.success && profileResult.body) {
                this.studentInfo = profileResult.body;
                localStorage.setItem('dkhp_student', JSON.stringify(this.studentInfo));
            }

            return {
                success: true,
                message: 'Đăng nhập thành công',
                studentInfo: this.studentInfo
            };
        }
        return loginResult;
    }

    async loginWithToken(token) {
        // Đăng nhập trực tiếp bằng token Bearer
        this.token = token;
        this._needsRelogin = false;
        this._reloginError = null;
        localStorage.setItem('dkhp_token', token);

        const profileResult = await this.request('GET', '/user/getSummaryProfile');
        if (profileResult.success !== false && profileResult.body) {
            this.studentInfo = profileResult.body;
            localStorage.setItem('dkhp_student', JSON.stringify(this.studentInfo));
            return {
                success: true,
                message: 'Đăng nhập thành công',
                studentInfo: this.studentInfo
            };
        }

        this.token = null;
        localStorage.removeItem('dkhp_token');
        return {
            success: false,
            message: profileResult.message || 'Token không hợp lệ hoặc đã hết hạn'
        };
    }

    async logout() {
        this.clearCredentials();
        this.clearSession();
        return { success: true, message: 'Đăng xuất thành công' };
    }

    clearSession() {
        this.token = null;
        this.studentInfo = null;
        this._needsRelogin = false;
        this._reloginError = null;
        localStorage.removeItem('dkhp_token');
        localStorage.removeItem('dkhp_student');
        window.location.href = '/';
    }

    async validateSession() {
        if (!this.token) return { success: false, message: 'Chưa đăng nhập' };

        // Test connection
        const result = await this.request('GET', '/dkhp/getHocPhanHocMoi?idDot=0');
        if (result.success !== false) {
            return { success: true, studentInfo: this.studentInfo };
        }
        return { success: false };
    }

    async getCurrentUser() {
        if (this.token && this.studentInfo) {
            return { success: true, studentInfo: this.studentInfo };
        }
        return { success: false };
    }

    // ==================== Events ====================

    parsePositiveInteger(value, label, optional = false) {
        const text = String(value ?? '').trim();
        if (!text && optional) return null;
        const number = Number(text);
        if (!/^\d+$/.test(text) || !Number.isSafeInteger(number) || number <= 0) {
            throw new Error(`${label} phải là số nguyên dương`);
        }
        return number;
    }

    buildEventRegistration(eventId, dateId = null, objectId = null) {
        const id = this.parsePositiveInteger(eventId, 'eventId');
        const date = this.parsePositiveInteger(dateId, 'dateId', true);
        const object = this.parsePositiveInteger(objectId, 'objectId', true);
        const payload = {};
        if (date !== null) payload.dateId = date;
        if (object !== null) payload.objectId = object;
        return { eventId: id, endpoint: `/eventWeb/register?eventId=${id}`, payload };
    }

    async registerEvent(eventId, dateId = null, objectId = null, shouldContinue = null, onProgress = null) {
        const { endpoint, payload } = this.buildEventRegistration(eventId, dateId, objectId);
        const result = await this.request('POST', endpoint, payload, false, shouldContinue, onProgress);
        const isSuccess = result?.success === true;
        return { ...result, success: isSuccess, message: result?.message || (isSuccess ? 'Đăng ký sự kiện thành công' : 'Portal chưa xác nhận đăng ký thành công') };
    }

    async cancelEvent(eventId) {
        const id = this.parsePositiveInteger(eventId, 'eventId');
        const result = await this.request('DELETE', `/eventWeb/cancel/${id}`);
        const isSuccess = result?.success === true;
        return { ...result, success: isSuccess, message: result?.message || (isSuccess ? 'Hủy đăng ký sự kiện thành công' : 'Portal chưa xác nhận hủy đăng ký thành công') };
    }

    // ==================== Courses ====================

    async getMainCourses(idDot) {
        const result = await this.request('GET', `/dkhp/getHocPhanHocMoi?idDot=${idDot}`);
        return { success: result.success !== false, data: result.body || [], message: result.message };
    }

    async getExtraCourses(idDot) {
        const result = await this.request('GET', `/dkhpdk/getHocPhanHocMoi?idDot=${idDot}`); // Note: dkhpdk
        return { success: result.success !== false, data: result.body || [], message: result.message };
    }

    async getImprovementCourses(idDot) {
        const result = await this.request('GET', `/dkhp/getHocPhanCaiThien?idDot=${idDot}`);
        return { success: result.success !== false, data: result.body || [], message: result.message };
    }

    async getRegisteredCourses(idDot) {
        const [mainResult, extraResult, improvementResult] = await Promise.all([
            this.request('GET', `/dkhp/getLHPDaDangKy?idDot=${idDot}`),
            this.request('GET', `/dkhpdk/getLHPDaDangKy?idDot=${idDot}`),
            // Handle 404 for improvement courses gracefully
            this.request('GET', `/dkhp/getLHPCaiThienDaDangKy?idDot=${idDot}`).then(res => {
                if (!res.success && (res.message?.includes('404') || res.message?.includes('Request failed'))) {
                    console.warn('Ignored 404 for improvement courses');
                    return { success: true, body: [] };
                }
                return res;
            })
        ]);

        return {
            success: true,
            data: {
                main: mainResult.body || [],
                extra: extraResult.body || [],
                improvement: improvementResult.body || []
            }
        };
    }

    // ==================== Classes ====================

    async getMainClasses(idDot, maHocPhan) {
        const result = await this.request('GET', `/dkhp/getLopHocPhanChoDangKy?idDot=${idDot}&maHocPhan=${encodeURIComponent(maHocPhan)}`);
        return { success: result.success !== false, data: result.body || [], message: result.message };
    }

    async getExtraClasses(idDot, maHocPhan) {
        const result = await this.request('GET', `/dkhpdk/getLopHocPhan?idDot=${idDot}&maHocPhan=${encodeURIComponent(maHocPhan)}`);
        return { success: result.success !== false, data: result.body || [], message: result.message };
    }

    async getImprovementClasses(idDot, maHocPhan) {
        const result = await this.request('GET', `/dkhp/getLopHocPhanChoDangKy?idDot=${idDot}&maHocPhan=${encodeURIComponent(maHocPhan)}`);
        return { success: result.success !== false, data: result.body || [], message: result.message };
    }

    async getClassDetail(classId, type = 'main') {
        let endpoint = `/dkhp/getLopHocPhanDetail?idLopHocPhan=${classId}`;
        if (type === 'extra') {
            endpoint = `/dkhpdk/getLopHocPhanDetail?idLopHocPhan=${classId}`;
        }
        const result = await this.request('GET', endpoint);
        return { success: result.success !== false, data: result.body || [], message: result.message };
    }

    async getSchedule(idDangKy) {
        const result = await this.request('GET', `/dkhp/getLichHoc?idDangKy=${idDangKy}`);
        return { success: result.success !== false, data: result.body || [], message: result.message };
    }

    // ==================== Registration ====================

    async registerMainClass(classId) {
        const result = await this.request('POST', `/dkhp/dangKyLopHocPhan?idLopHocPhan=${classId}`);
        const isSuccess = result.success === true;
        return { ...result, success: isSuccess, message: result.message || (isSuccess ? 'Đăng ký thành công' : 'Không xác định được kết quả đăng ký') };
    }

    async registerExtraClass(classId) {
        const result = await this.request('POST', `/dkhpdk/dangKyLopHocPhan?idLopHocPhan=${classId}`);
        const isSuccess = result.success === true;
        return { ...result, success: isSuccess, message: result.message || (isSuccess ? 'Đăng ký thành công' : 'Không xác định được kết quả đăng ký') };
    }

    async registerImprovementClass(classId) {
        const result = await this.request('POST', `/dkhp/dangKyLopHocPhan?idLopHocPhan=${classId}`);
        const isSuccess = result.success === true;
        return { ...result, success: isSuccess, message: result.message || (isSuccess ? 'Đăng ký thành công' : 'Không xác định được kết quả đăng ký') };
    }

    async cancelRegistration(idDangKy) {
        const result = await this.request('DELETE', `/dkhp/huyDangKy?idDangKy=${idDangKy}`);
        return { success: result.success !== false, message: result.message || 'Hủy thành công' };
    }

}

// Create global instance
const api = new ApiClient();
