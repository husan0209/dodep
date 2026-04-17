// Simple mock API server for testing frontend
const express = require('express');
const cors = require('cors');
const crypto = require('crypto');

const app = express();
const PORT = 8080;

// Mock database
const users = new Map();
const sessions = new Map();
const blacklistedTokens = new Set();

// Mock JWT
function generateToken(payload, expiresIn = '1h') {
  return crypto.randomBytes(32).toString('hex');
}

app.use(cors());
app.use(express.json());

// Health check
app.get('/health', (req, res) => {
  res.send('ok');
});

app.get('/ready', (req, res) => {
  res.send('ready');
});

// Auth routes
app.post('/api/v1/auth/register', (req, res) => {
  console.log('Register request body:', req.body);
  const { email: rawEmail, password, username, country_code, currency_code } = req.body;
  
  // Clean email from whitespace (same as frontend)
  const email = rawEmail ? rawEmail.replace(/\s/g, '') : '';
  
  console.log('Cleaned email:', email);

  if (!email || !password || !username) {
    console.log('Missing fields:', { email: !!email, password: !!password, username: !!username });
    return res.status(400).json({
      code: 'VALIDATION_ERROR',
      message: 'Email, password, and username are required',
      request_id: crypto.randomUUID(),
    });
  }

  if (users.has(email)) {
    return res.status(400).json({
      code: 'USER_ALREADY_EXISTS',
      message: 'User with this email already exists',
      request_id: crypto.randomUUID(),
    });
  }

  const user = {
    id: users.size + 1,
    uuid: crypto.randomUUID(),
    email,
    username,
    phone: null,
    country: country_code || 'RU',
    currency: currency_code || 'RUB',
    kyc_level: 0,
    status: 'active',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    last_login_at: null,
  };

  users.set(email, { ...user, password });

  const accessToken = generateToken({ userId: user.id, email });
  const refreshToken = generateToken({ userId: user.id, type: 'refresh' });

  sessions.set(refreshToken, {
    userId: user.id,
    expiresAt: Date.now() + 7 * 24 * 60 * 60 * 1000, // 7 days
  });

  res.status(201).json({
    access_token: accessToken,
    refresh_token: refreshToken,
    expires_in: 3600,
    token_type: 'Bearer',
    user,
  });
});

app.post('/api/v1/auth/login', (req, res) => {
  const { email: rawEmail, password } = req.body;
  
  // Clean email from whitespace (same as frontend)
  const email = rawEmail ? rawEmail.replace(/\s/g, '') : '';
  
  console.log('Login request - cleaned email:', email);

  if (!email || !password) {
    return res.status(400).json({
      code: 'VALIDATION_ERROR',
      message: 'Email and password are required',
      request_id: crypto.randomUUID(),
    });
  }

  const userRecord = users.get(email);

  if (!userRecord || userRecord.password !== password) {
    return res.status(401).json({
      code: 'INVALID_CREDENTIALS',
      message: 'Invalid email or password',
      request_id: crypto.randomUUID(),
    });
  }

  const user = { ...userRecord };
  delete user.password;
  user.last_login_at = new Date().toISOString();
  users.set(email, userRecord);

  const accessToken = generateToken({ userId: user.id, email });
  const refreshToken = generateToken({ userId: user.id, type: 'refresh' });

  sessions.set(refreshToken, {
    userId: user.id,
    expiresAt: Date.now() + 7 * 24 * 60 * 60 * 1000,
  });

  res.json({
    access_token: accessToken,
    refresh_token: refreshToken,
    expires_in: 3600,
    token_type: 'Bearer',
    user,
  });
});

app.post('/api/v1/auth/logout', (req, res) => {
  const { refresh_token } = req.body;

  if (refresh_token) {
    sessions.delete(refresh_token);
    blacklistedTokens.add(refresh_token);
  }

  res.json({ success: true });
});

app.get('/api/v1/auth/me', (req, res) => {
  const authHeader = req.headers.authorization;

  if (!authHeader || !authHeader.startsWith('Bearer ')) {
    return res.status(401).json({
      code: 'UNAUTHORIZED',
      message: 'Missing or invalid authorization header',
      request_id: crypto.randomUUID(),
    });
  }

  // For mock, just return a test user
  const testUser = {
    id: 1,
    uuid: crypto.randomUUID(),
    email: 'test@example.com',
    username: 'testuser',
    phone: null,
    country: 'US',
    currency: 'USD',
    kyc_level: 0,
    status: 'active',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    last_login_at: new Date().toISOString(),
  };

  res.json(testUser);
});

app.post('/api/v1/auth/refresh', (req, res) => {
  const { refresh_token } = req.body;

  if (!refresh_token) {
    return res.status(401).json({
      code: 'MISSING_REFRESH_TOKEN',
      message: 'Refresh token is required',
      request_id: crypto.randomUUID(),
    });
  }

  const session = sessions.get(refresh_token);

  if (!session || session.expiresAt < Date.now()) {
    sessions.delete(refresh_token);
    return res.status(401).json({
      code: 'INVALID_REFRESH_TOKEN',
      message: 'Invalid or expired refresh token',
      request_id: crypto.randomUUID(),
    });
  }

  const accessToken = generateToken({ userId: session.userId });
  const newRefreshToken = generateToken({ userId: session.userId, type: 'refresh' });

  sessions.delete(refresh_token);
  sessions.set(newRefreshToken, session);

  res.json({
    access_token: accessToken,
    refresh_token: newRefreshToken,
    expires_in: 3600,
    token_type: 'Bearer',
  });
});

app.listen(PORT, () => {
  console.log(`🚀 Mock API Server running on http://localhost:${PORT}`);
  console.log(`   Health: http://localhost:${PORT}/health`);
  console.log(`   Auth:   http://localhost:${PORT}/api/v1/auth/*`);
});
