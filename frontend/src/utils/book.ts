const readingStatusLabels: Record<string, string> = {
  unread: '未读',
  reading: '在读',
  finished: '已读',
  paused: '暂停',
  abandoned: '弃读',
};

export function readingStatusLabel(status: string): string {
  return readingStatusLabels[status] ?? status;
}
