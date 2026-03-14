const API_URL = 'http://localhost:3000/api';

// State
let state = {
    token: localStorage.getItem('token') || null,
    tempToken: null,
    username: null
};

// UI Elements
const views = document.querySelectorAll('.view');
const toastEl = document.getElementById('toast');

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    // Check if logged in
    if (state.token) {
        checkTokenAndRedirect();
    } else {
        showView('view-auth');
    }

    setupEventListeners();
    setupCodeInputs();
});

// View Navigation
function showView(viewId) {
    views.forEach(v => {
        v.classList.remove('active');
        // minor reset timeout for animation
        setTimeout(() => {
            if (!v.classList.contains('active')) v.style.display = 'none';
        }, 400);
    });

    const view = document.getElementById(viewId);
    view.style.display = 'block';
    setTimeout(() => view.classList.add('active'), 10);
}

function showToast(message, type = 'success') {
    toastEl.textContent = message;
    toastEl.className = `toast ${type} show`;
    setTimeout(() => {
        toastEl.classList.remove('show');
    }, 3000);
}

function setupEventListeners() {
    // Auth Form (Login)
    document.getElementById('auth-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const username = document.getElementById('username').value;
        const password = document.getElementById('password').value;
        state.username = username;

        try {
            const res = await fetch(`${API_URL}/login`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, password })
            });
            const data = await res.json();

            if (res.ok) {
                if (data.requireMfa) {
                    state.tempToken = data.tempToken;
                    showView('view-mfa-verify');
                    focusFirstCodeInput('view-mfa-verify');
                } else {
                    state.token = data.token;
                    localStorage.setItem('token', state.token);
                    // Generate MFA setup immediately since requirement says "require use enable MFA before allow go to dashboard"
                    initMfaSetup();
                }
            } else {
                showToast(data.error, 'error');
            }
        } catch (err) {
            showToast('Connection error', 'error');
        }
    });

    // Auth Form (Register)
    document.getElementById('btn-register').addEventListener('click', async () => {
        const username = document.getElementById('username').value;
        const password = document.getElementById('password').value;

        if (!username || !password) {
            return showToast('Username and password required', 'error');
        }

        try {
            const res = await fetch(`${API_URL}/register`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, password })
            });
            const data = await res.json();
            if (res.ok) {
                showToast('Registered successfully! Now logging in...', 'success');
            } else {
                showToast(data.error, 'error');
            }
        } catch (err) {
            showToast('Connection error', 'error');
        }
    });

    // MFA Verify (Login Step)
    document.getElementById('mfa-verify-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const code = getCodeFromInputs('mfa-verify-inputs');
        if (code.length !== 6) return showToast('Enter 6-digit code', 'error');

        try {
            const res = await fetch(`${API_URL}/mfa/verify`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ tempToken: state.tempToken, code })
            });
            const data = await res.json();

            if (res.ok) {
                state.token = data.token;
                state.tempToken = null;
                localStorage.setItem('token', state.token);
                loadDashboard();
            } else {
                showToast(data.error, 'error');
                clearCodeInputs('mfa-verify-inputs');
            }
        } catch (err) {
            showToast('Connection error', 'error');
        }
    });

    // MFA Setup Verification
    document.getElementById('mfa-setup-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const code = getCodeFromInputs('mfa-setup-inputs');
        if (code.length !== 6) return showToast('Enter 6-digit code', 'error');

        try {
            const res = await fetch(`${API_URL}/mfa/verify`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ token: state.token, code })
            });
            const data = await res.json();

            if (res.ok) {
                showToast('MFA enabled successfully!', 'success');
                loadDashboard();
            } else {
                showToast(data.error, 'error');
                clearCodeInputs('mfa-setup-inputs');
            }
        } catch (err) {
            showToast('Connection error', 'error');
        }
    });
}

// Code Input Logic
function setupCodeInputs() {
    const inputContainers = document.querySelectorAll('.code-inputs');
    inputContainers.forEach(container => {
        const inputs = container.querySelectorAll('.code-digit');
        inputs.forEach((input, index) => {
            // Auto focus next input
            input.addEventListener('input', (e) => {
                if (e.target.value.length === 1 && index < inputs.length - 1) {
                    inputs[index + 1].focus();
                }
            });
            // Handle backspace
            input.addEventListener('keydown', (e) => {
                if (e.key === 'Backspace' && !e.target.value && index > 0) {
                    inputs[index - 1].focus();
                }
            });
            // Handle paste
            input.addEventListener('paste', (e) => {
                e.preventDefault();
                const pastedData = e.clipboardData.getData('text').slice(0, 6).replace(/\D/g, '');
                if (pastedData) {
                    for (let i = 0; i < pastedData.length; i++) {
                        if (inputs[i]) {
                            inputs[i].value = pastedData[i];
                        }
                    }
                    if (inputs[pastedData.length - 1] && pastedData.length < 6) {
                        inputs[pastedData.length].focus();
                    } else {
                        inputs[5].focus();
                    }
                }
            });
        });
    });
}

function getCodeFromInputs(containerId) {
    const inputs = document.querySelectorAll(`#${containerId} .code-digit`);
    let code = '';
    inputs.forEach(i => code += i.value);
    return code;
}

function clearCodeInputs(containerId) {
    const inputs = document.querySelectorAll(`#${containerId} .code-digit`);
    inputs.forEach(i => i.value = '');
    inputs[0].focus();
}
function focusFirstCodeInput(viewId) {
    setTimeout(() => {
        const input = document.querySelector(`#${viewId} .code-digit`);
        if (input) input.focus();
    }, 100);
}

// Actions
async function checkTokenAndRedirect() {
    try {
        const res = await fetch(`${API_URL}/me`, {
            headers: { 'Authorization': `Bearer ${state.token}` }
        });
        const data = await res.json();
        if (res.ok) {
            state.username = data.username;
            if (data.mfaEnabled) {
                loadDashboard();
            } else {
                initMfaSetup();
            }
        } else {
            logout(); // invalidate token
        }
    } catch {
        showView('view-auth');
    }
}

async function initMfaSetup() {
    showView('view-mfa-setup');
    try {
        const res = await fetch(`${API_URL}/mfa/generate`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ token: state.token })
        });
        const data = await res.json();
        if (res.ok) {
            document.getElementById('qr-image').src = data.qrCodeUrl;
            document.getElementById('secret-text').textContent = data.secret;
            focusFirstCodeInput('view-mfa-setup');
        } else {
            showToast(data.error, 'error');
            if (data.error === "MFA already enabled") {
                loadDashboard();
            }
        }
    } catch (err) {
        showToast('Error generating MFA', 'error');
    }
}

async function loadDashboard() {
    showView('view-dashboard');
    try {
        const res = await fetch(`${API_URL}/dashboard`, {
            headers: { 'Authorization': `Bearer ${state.token}` }
        });
        const data = await res.json();
        if (res.ok) {
            document.getElementById('dashboard-message').textContent = data.message;
        } else {
            document.getElementById('dashboard-message').textContent = data.error;
            if (data.error.includes("Must enable MFA")) {
                initMfaSetup();
            }
        }
    } catch (err) {
        document.getElementById('dashboard-message').textContent = 'Error loading dashboard.';
    }
}

function logout() {
    state.token = null;
    state.tempToken = null;
    state.username = null;
    localStorage.removeItem('token');
    clearCodeInputs('mfa-verify-inputs');
    clearCodeInputs('mfa-setup-inputs');
    document.getElementById('username').value = '';
    document.getElementById('password').value = '';
    showView('view-auth');
}
