const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080/api';

interface ApiOptions extends RequestInit {
  skipAuth?: boolean;
}

let accessToken: string | null = null;

export function setAccessToken(token: string | null) {
  accessToken = token;
}

export async function apiRequest<T>(path: string, options: ApiOptions = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string> || {}),
  };

  if (!options.skipAuth && accessToken) {
    headers['Authorization'] = `Bearer ${accessToken}`;
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
    credentials: 'include', // для refresh cookie
  });

  if (response.status === 401 && !options.skipAuth) {
    // Пробуем refresh
    const refreshed = await tryRefresh();
    if (refreshed) {
      return apiRequest<T>(path, options);
    }
    throw new Error('Unauthorized');
  }

  if (!response.ok) {
    const err = await response.json().catch(() => ({}));
    throw new Error(err.message || `HTTP ${response.status}`);
  }

  return response.json();
}

async function tryRefresh(): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/auth/refresh`, {
      method: 'POST',
      credentials: 'include',
    });
    if (res.ok) {
      const data = await res.json();
      setAccessToken(data.accessToken);
      return true;
    }
  } catch {}
  return false;
}
