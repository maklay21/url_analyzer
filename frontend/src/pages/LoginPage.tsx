import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '../stores/authStore';

export function LoginPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [isRegister, setIsRegister] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const login = useAuthStore((s) => s.login);
  const navigate = useNavigate();

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      // В зависимости от режима — логин или регистрация.
      // В вашем authStore.login шлёт на /auth/login.
      // Для регистрации сделаем отдельный запрос здесь.
      if (isRegister) {
        const res = await fetch('http://localhost:8080/api/auth/register', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          credentials: 'include',
          body: JSON.stringify({ email, password }),
        });
        if (!res.ok) {
          const data = await res.json().catch(() => ({}));
          throw new Error(data.message || 'Ошибка регистрации');
        }
        const data = await res.json();
        // Сохраняем access-токен и логинимся
        const { setAccessToken } = await import('../api/client');
        setAccessToken(data.accessToken);
        // После регистрации просто логинимся
      }

      await login(email, password);
      navigate('/');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div style={{ maxWidth: 400, margin: '80px auto', fontFamily: 'sans-serif' }}>
      <h1>{isRegister ? 'Регистрация' : 'Вход'}</h1>

      <form onSubmit={handleSubmit}>
        <input
          type="email"
          placeholder="Email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          style={{ width: '100%', padding: 10, marginBottom: 10, fontSize: 16 }}
        />
        <input
          type="password"
          placeholder="Пароль (минимум 8 символов)"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          minLength={8}
          style={{ width: '100%', padding: 10, marginBottom: 10, fontSize: 16 }}
        />
        <button
          type="submit"
          disabled={loading}
          style={{ width: '100%', padding: 10, fontSize: 16, cursor: 'pointer' }}
        >
          {loading ? 'Загрузка...' : isRegister ? 'Зарегистрироваться' : 'Войти'}
        </button>
      </form>

      {error && <p style={{ color: 'red', marginTop: 10 }}>{error}</p>}

      <p style={{ marginTop: 20, textAlign: 'center' }}>
        {isRegister ? 'Уже есть аккаунт?' : 'Нет аккаунта?'}{' '}
        <button
          onClick={() => setIsRegister(!isRegister)}
          style={{
            background: 'none',
            border: 'none',
            color: '#0066cc',
            cursor: 'pointer',
            textDecoration: 'underline',
            fontSize: 14,
          }}
        >
          {isRegister ? 'Войти' : 'Зарегистрироваться'}
        </button>
      </p>
    </div>
  );
}
