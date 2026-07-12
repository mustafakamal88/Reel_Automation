export function formatValue(value: unknown): string {
  if (value == null || value === '') return 'Unavailable';
  if (typeof value === 'number') return Number.isInteger(value) ? value.toLocaleString() : value.toFixed(3);
  if (typeof value === 'string') return value;
  return JSON.stringify(value);
}

export function formatMetric(label: string, value: unknown): string {
  if (value == null || value === '') return 'Unavailable';
  const normalized = label.toLowerCase();
  if (typeof value === 'number') {
    if (normalized.includes('rate') || normalized.includes('concentration')) {
      return `${(value * 100).toFixed(1)}%`;
    }
    if (normalized.includes('confidence')) {
      const percent = value <= 1 ? value * 100 : value;
      return `${percent.toFixed(1)}%`;
    }
    if (normalized.includes('per_1000') || normalized.includes('per 1000')) {
      return value.toFixed(1);
    }
    if (!Number.isInteger(value) && normalized.includes('day')) {
      return value.toLocaleString(undefined, { maximumFractionDigits: 1 });
    }
    return Number.isInteger(value) ? value.toLocaleString() : value.toFixed(2);
  }
  if (Array.isArray(value)) return value.join(' · ');
  return formatValue(value);
}

export function formatLabel(label: string): string {
  const preferred: Record<string, string> = {
    views_per_day: 'Views/day',
    likes_per_1000_views: 'Likes per 1,000 views',
    comments_per_1000_views: 'Comments per 1,000 views',
    engagement_rate: 'Engagement rate',
    velocity_label: 'Velocity',
    age_days: 'Age in days',
    average_views: 'Average views',
    median_views: 'Median views',
    max_views: 'Max views',
    min_views: 'Min views',
    sample_size: 'Sample size',
    view_concentration: 'View concentration',
    outlier_videos: 'Outlier videos',
  };
  if (preferred[label]) return preferred[label];
  return label.replaceAll('_', ' ').replace(/\b\w/g, char => char.toUpperCase());
}
