import { useEffect, useState } from 'react';

const shanghaiDateFormatter = new Intl.DateTimeFormat('en-CA', {
  timeZone: 'Asia/Shanghai',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
});

export function getShanghaiISODate(date = new Date()): string {
  const parts = new Map(
    shanghaiDateFormatter.formatToParts(date).map(({ type, value }) => [type, value]),
  );
  return `${parts.get('year')}-${parts.get('month')}-${parts.get('day')}`;
}

// Keeps long-lived teacher and student pages aligned with the server's
// Asia/Shanghai calendar even when they remain open through midnight.
export function useShanghaiDate(): string {
  const [today, setToday] = useState(() => getShanghaiISODate());

  useEffect(() => {
    const refresh = () => {
      const next = getShanghaiISODate();
      setToday((current) => current === next ? current : next);
    };
    const interval = window.setInterval(refresh, 60_000);
    window.addEventListener('focus', refresh);
    return () => {
      window.clearInterval(interval);
      window.removeEventListener('focus', refresh);
    };
  }, []);

  return today;
}
