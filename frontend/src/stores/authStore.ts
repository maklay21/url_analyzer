import { create } from 'zustand';
import { apiRequest, setAccessToken } from '../api/client';

interface User {
  id: string;
  email: string;
}

interface AuthState {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  bootstrap: () => Promise<void>;
}

// Защита от параллельных вызовов bootstrap
let bootstrapPromise: Promise<void> | null = null;

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isLoading: true,
  isAuthenticated: false,

  login: async (email, password) => {
    const data = await apiRequest<{ accessToken: string; userId: string }>(
      '/auth/login',
      { method: 'POST', body: JSON.stringify({ email, password }), skipAuth: true }
    );
    setAccessToken(data.accessToken);
    set({
      user: { id: data.userId, email },
      isAuthenticated: true,
      isLoading: false,
    });
  },

  logout: async () => {
    try {
      await apiRequest('/auth/logout', { method: 'POST' });
    } finally {
      setAccessToken(null);
      set({ user: null, isAuthenticated: false, isLoading: false });
    }
  },

  bootstrap: async () => {
    if (bootstrapPromise) return bootstrapPromise;

    bootstrapPromise = (async () => {
      try {
        const data = await apiRequest<{ accessToken: string; userId: string }>(
          '/auth/refresh',
          { method: 'POST', skipAuth: true }
        );
        setAccessToken(data.accessToken);

        // Пробуем получить email из /auth/me, но не падаем, если там заглушка
        let user: User = { id: data.userId, email: '' };
        try {
          const me = await apiRequest<User>('/auth/me');
          if (me && me.id) user = me;
        } catch {
          // /auth/me ещё не реализован — не критично
        }

        set({ user, isAuthenticated: true, isLoading: false });
      } catch {
        set({ isLoading: false, isAuthenticated: false });
      } finally {
        bootstrapPromise = null;
      }
    })();

    return bootstrapPromise;
  },
}));