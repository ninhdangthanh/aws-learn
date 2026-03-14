const express = require('express');
const cors = require('cors');
const speakeasy = require('speakeasy');
const QRCode = require('qrcode');
const crypto = require('crypto');
const path = require('path');

const app = express();
app.use(cors());
app.use(express.json());
app.use(express.static(path.join(__dirname, '../frontend')));


// In-memory data store
const users = {}; // { username: { password, mfaEnabled, mfaSecret } }
const sessions = {}; // { token: username }
const tempSessions = {}; // { tempToken: username }

// Helper to generate a token
const generateToken = () => crypto.randomBytes(32).toString('hex');

// Register
app.post('/api/register', (req, res) => {
    const { username, password } = req.body;
    if (!username || !password) {
        return res.status(400).json({ error: 'Username and password required' });
    }
    if (users[username]) {
        return res.status(400).json({ error: 'User already exists' });
    }
    users[username] = {
        password, // In a real app, hash this!
        mfaEnabled: false,
        mfaSecret: null
    };
    res.json({ message: 'User registered successfully' });
});

// Login
app.post('/api/login', (req, res) => {
    const { username, password } = req.body;
    const user = users[username];

    if (!user || user.password !== password) {
        return res.status(401).json({ error: 'Invalid credentials' });
    }

    if (user.mfaEnabled) {
        // Require MFA code
        const tempToken = generateToken();
        tempSessions[tempToken] = username;
        return res.json({ requireMfa: true, tempToken });
    } else {
        // No MFA required yet, but we will force the user to set it up before dashboard
        // Let's log them in, but they MUST go to MFA setup phase first.
        const token = generateToken();
        sessions[token] = username;
        return res.json({ requireMfa: false, token });
    }
});

// Generate MFA Setup
app.post('/api/mfa/generate', async (req, res) => {
    const { token } = req.body;
    const username = sessions[token];
    if (!username) {
        return res.status(401).json({ error: 'Unauthorized' });
    }

    const user = users[username];
    if (user.mfaEnabled) {
        return res.status(400).json({ error: 'MFA already enabled' });
    }

    // Generate a new secret
    const secret = speakeasy.generateSecret({
        name: `MFA Example App (${username})`
    });

    // Save secret temporarily (or overwrite existing pending secret)
    user.mfaSecret = secret.base32;

    try {
        const qrCodeUrl = await QRCode.toDataURL(secret.otpauth_url);
        res.json({
            secret: secret.base32,
            qrCodeUrl
        });
    } catch (err) {
        res.status(500).json({ error: 'Error generating QR Code' });
    }
});

// Verify and Enable MFA OR Verify during Login
app.post('/api/mfa/verify', (req, res) => {
    const { token, tempToken, code } = req.body;
    
    // Check if it's a login verification
    if (tempToken) {
        const username = tempSessions[tempToken];
        if (!username) return res.status(401).json({ error: 'Invalid or expired temporary session' });

        const user = users[username];
        const verified = speakeasy.totp.verify({
            secret: user.mfaSecret,
            encoding: 'base32',
            token: code,
            window: 1 // Allow 1 step before or after
        });

        if (verified) {
            delete tempSessions[tempToken];
            const newToken = generateToken();
            sessions[newToken] = username;
            return res.json({ token: newToken });
        } else {
            return res.status(400).json({ error: 'Invalid MFA code' });
        }
    }

    // Check if it's an initial setup verification
    if (token) {
        const username = sessions[token];
        if (!username) return res.status(401).json({ error: 'Unauthorized' });

        const user = users[username];
        
        const verified = speakeasy.totp.verify({
            secret: user.mfaSecret,
            encoding: 'base32',
            token: code,
            window: 1
        });

        if (verified) {
            user.mfaEnabled = true;
            return res.json({ message: 'MFA enabled successfully' });
        } else {
            return res.status(400).json({ error: 'Invalid MFA code' });
        }
    }

    return res.status(400).json({ error: 'Provide token or tempToken' });
});

// Dashboard
app.get('/api/dashboard', (req, res) => {
    // Get token from Authorization header (Bearer <token>)
    const authHeader = req.headers.authorization;
    if (!authHeader) return res.status(401).json({ error: 'Unauthorized' });

    const token = authHeader.split(' ')[1];
    const username = sessions[token];

    if (!username) {
        return res.status(401).json({ error: 'Unauthorized' });
    }

    const user = users[username];
    if (!user.mfaEnabled) {
        return res.status(403).json({ error: 'MFA not enabled. Must enable MFA to view dashboard.' });
    }

    res.json({ message: `Hello ${username}, welcome to the secure dashboard!` });
});

app.get('/api/me', (req, res) => {
    const authHeader = req.headers.authorization;
    if (!authHeader) return res.status(401).json({ error: 'Unauthorized' });

    const token = authHeader.split(' ')[1];
    const username = sessions[token];

    if (!username) {
        return res.status(401).json({ error: 'Unauthorized' });
    }

    const user = users[username];
    res.json({ username, mfaEnabled: user.mfaEnabled });
});

const PORT = 3000;
app.listen(PORT, () => {
    console.log(`Backend listening on port ${PORT}`);
});
