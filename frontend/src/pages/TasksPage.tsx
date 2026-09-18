import { useEffect, useState } from 'react';
import { apiRequest } from '../api/client';

interface TaskItem {
  taskId: string;
  url: string;
  domain: string;
  status: string;
  summary: string;
  createdAt: string;
}

export function TasksPage() {
  const [tasks, setTasks] = useState<TaskItem[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadTasks();
    const interval = setInterval(loadTasks, 5000); // автоподгрузка
    return () => clearInterval(interval);
  }, []);

  async function loadTasks() {
    try {
      const data = await apiRequest<{ tasks: TaskItem[] }>('/tasks?page=1&pageSize=50');
      setTasks(data.tasks || []);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  }

  const statusLabels: Record<string, string> = {
    processing: 'Обработка',
    completed: 'Готово',
    failed: 'Ошибка',
    insufficient_data: 'Мало данных',
  };

  return (
    <div style={{ maxWidth: 1000, margin: '50px auto' }}>
      <h1>Результаты анализа</h1>
      {loading ? <p>Загрузка...</p> : (
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr>
              <th style={th}>URL</th>
              <th style={th}>Домен</th>
              <th style={th}>Статус</th>
              <th style={th}>Резюме</th>
              <th style={th}>Дата</th>
            </tr>
          </thead>
          <tbody>
            {tasks.map((t) => (
              <tr key={t.taskId}>
                <td style={td}>{t.url}</td>
                <td style={td}>{t.domain}</td>
                <td style={td}>{statusLabels[t.status] || t.status}</td>
                <td style={td}>{t.summary || '—'}</td>
                <td style={td}>{t.createdAt}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

const th: React.CSSProperties = { textAlign: 'left', padding: 8, borderBottom: '2px solid #ccc' };
const td: React.CSSProperties = { padding: 8, borderBottom: '1px solid #eee' };
