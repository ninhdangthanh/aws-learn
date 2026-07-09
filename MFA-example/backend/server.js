const express = require('express');
const cors = require('cors');
const speakeasy = require('speakeasy');
const QRCode = require('qrcode');
const crypto = require('crypto');
const path = require('path');
const {
  generateRegistrationOptions,
  verifyRegistrationResponse,
  generateAuthenticationOptions,
  verifyAuthenticationResponse,
} = require('@simplewebauthn/server');
const { isoBase64URL } = require('@simplewebauthn/server/helpers');

const app = express();
app.use(cors());
app.use(express.json());
app.use(express.static(path.join(__dirname, '../frontend')));

// RP (Relying Party) settings
const rpName = 'MFA Example App';
const rpID = 'localhost';
const origin = `http://${rpID}:3000`;

// In-memory data store
const users = {}; // { username: { id: "uuid", password, mfaEnabled, mfaSecret, currentChallenge, passkeys: [] } }
const sessions = {}; // { token: username }
const tempSessions = {}; // { tempToken: username }

// Helper to generate a token
const generateToken = () => crypto.randomBytes(32).toString('hex');
// Helper to generate User ID
const generateUserId = () => crypto.randomBytes(32).toString('base64url');

// --- User Registration & Login ---

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
        id: generateUserId(),
        password, // In a real app, hash this!
        mfaEnabled: false,
        mfaSecret: null,
        currentChallenge: null,
        passkeys: []
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

    if (user.mfaEnabled || user.passkeys.length > 0) {
        // Require MFA setup verification code (either passkey or TOTP)
        const tempToken = generateToken();
        tempSessions[tempToken] = username;
        return res.json({ 
            requireMfa: true, 
            tempToken,
            hasTotp: user.mfaEnabled,
            hasPasskey: user.passkeys.length > 0
        });
    } else {
        // No MFA required yet, but MUST go to MFA setup phase first.
        const token = generateToken();
        sessions[token] = username;
        return res.json({ requireMfa: false, token });
    }
});

// --- TOTP MFA ---

// Generate MFA Setup
app.post('/api/mfa/generate', async (req, res) => {
    const { token } = req.body;
    const username = sessions[token];
    if (!username) {
        return res.status(401).json({ error: 'Unauthorized' });
    }

    const user = users[username];
    if (user.mfaEnabled) {
        return res.status(400).json({ error: 'TOTP already enabled' });
    }

    const secret = speakeasy.generateSecret({
        name: `MFA Example App (${username})`
    });

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

// Verify TOTP MFA (Initial Setup or Login)
app.post('/api/mfa/verify', (req, res) => {
    const { token, tempToken, code } = req.body;
    
    // Check if it's a login verification
    if (tempToken) {
        const username = tempSessions[tempToken];
        if (!username) return res.status(401).json({ error: 'Invalid or expired temporary session' });

        const user = users[username];
        if (!user.mfaEnabled) return res.status(400).json({ error: 'TOTP not enabled' });
        
        const verified = speakeasy.totp.verify({
            secret: user.mfaSecret,
            encoding: 'base32',
            token: code,
            window: 1
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
            return res.json({ message: 'TOTP MFA enabled successfully' });
        } else {
            return res.status(400).json({ error: 'Invalid MFA code' });
        }
    }

    return res.status(400).json({ error: 'Provide token or tempToken' });
});

// --- Passkeys / WebAuthn ---

// Generate Registration Options (Setup Phase)
app.post('/api/passkey/generate-registration-options', async (req, res) => {
    const authHeader = req.headers.authorization;
    if (!authHeader) return res.status(401).json({ error: 'Unauthorized' });

    const token = authHeader.split(' ')[1];
    const username = sessions[token];

    if (!username) return res.status(401).json({ error: 'Unauthorized' });

    const user = users[username];

    const options = await generateRegistrationOptions({
        rpName,
        rpID,
        userID: user.id,
        userName: username,
        attestationType: 'none',
        excludeCredentials: user.passkeys.map(passkey => ({
            id: passkey.credentialID,
            type: 'public-key',
            transports: passkey.transports,
        })),
        authenticatorSelection: {
            residentKey: 'preferred',
            userVerification: 'preferred',
            authenticatorAttachment: 'platform',
        },
    });

    user.currentChallenge = options.challenge;
    res.json(options);
});

// Verify Registration Response (Setup Phase)
app.post('/api/passkey/verify-registration', async (req, res) => {
    const authHeader = req.headers.authorization;
    if (!authHeader) return res.status(401).json({ error: 'Unauthorized' });

    const token = authHeader.split(' ')[1];
    const username = sessions[token];
    if (!username) return res.status(401).json({ error: 'Unauthorized' });

    const user = users[username];
    const body = req.body;

    let verification;
    try {
        verification = await verifyRegistrationResponse({
            response: body,
            expectedChallenge: user.currentChallenge,
            expectedOrigin: origin,
            expectedRPID: rpID,
        });
    } catch (error) {
        console.error(error);
        return res.status(400).json({ error: error.message });
    }

    const { verified, registrationInfo } = verification;
    
    if (verified && registrationInfo) {
        const { credentialPublicKey, credentialID, counter } = registrationInfo;

        const existingPasskey = user.passkeys.find(passkey =>
            passkey.credentialID.toString() === credentialID.toString()
        );

        if (!existingPasskey) {
            user.passkeys.push({
                credentialID,
                credentialPublicKey,
                counter,
                transports: body.response.transports || [],
            });
        }
        res.json({ verified: true });
    } else {
        res.status(400).json({ error: 'Verification failed' });
    }
});

// Generate Authentication Options (Login Phase)
app.post('/api/passkey/generate-authentication-options', async (req, res) => {
    const { tempToken } = req.body;
    const username = tempSessions[tempToken];

    if (!username) {
        return res.status(401).json({ error: 'Invalid or expired temporary session' });
    }

    const user = users[username];

    const options = await generateAuthenticationOptions({
        rpID,
        allowCredentials: user.passkeys.map(passkey => ({
            id: passkey.credentialID,
            type: 'public-key',
            transports: passkey.transports,
        })),
        userVerification: 'preferred',
    });

    user.currentChallenge = options.challenge;
    
    res.json(options);
});

// Verify Authentication Response (Login Phase)
app.post('/api/passkey/verify-authentication', async (req, res) => {
    const { tempToken, response } = req.body;
    const username = tempSessions[tempToken];

    if (!username || !response) {
        return res.status(401).json({ error: 'Invalid session or missing response' });
    }
    
    const user = users[username];
    // credentialID là Uint8Array; .toString('base64url') bỏ qua tham số nên không bao giờ khớp.
    const passkey = user.passkeys.find(p => isoBase64URL.fromBuffer(p.credentialID) === response.id);
    if (!passkey) {
        return res.status(400).json({ error: 'Could not find passkey for user' });
    }

    let verification;
    try {
        verification = await verifyAuthenticationResponse({
            response,
            expectedChallenge: user.currentChallenge,
            expectedOrigin: origin,
            expectedRPID: rpID,
            authenticator: {
                credentialPublicKey: passkey.credentialPublicKey,
                credentialID: passkey.credentialID,
                counter: passkey.counter,
            },
        });
    } catch (error) {
        console.error(error);
        return res.status(400).json({ error: error.message });
    }

    const { verified, authenticationInfo } = verification;
    
    if (verified) {
        passkey.counter = authenticationInfo.newCounter;
        
        delete tempSessions[tempToken];
        const token = generateToken();
        sessions[token] = username;
        
        res.json({ verified: true, token });
    } else {
        res.status(400).json({ error: 'Authentication failed' });
    }
});

// --- Dashboard & User Status ---

app.get('/api/dashboard', (req, res) => {
    const authHeader = req.headers.authorization;
    if (!authHeader) return res.status(401).json({ error: 'Unauthorized' });

    const token = authHeader.split(' ')[1];
    const username = sessions[token];

    if (!username) {
        return res.status(401).json({ error: 'Unauthorized' });
    }

    const user = users[username];
    if (!user.mfaEnabled && user.passkeys.length === 0) {
        return res.status(403).json({ error: 'MFA not configured. Must enable TOTP or Passkey to view dashboard.' });
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
    res.json({ 
        username, 
        mfaEnabled: user.mfaEnabled,
        passkeysEnabled: user.passkeys.length > 0
    });
});

const PORT = 3000;
app.listen(PORT, () => {
    console.log(`Backend listening on port ${PORT}`);
});
