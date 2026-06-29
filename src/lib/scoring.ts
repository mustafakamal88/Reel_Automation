import type { RiskLevel } from '../types';

export interface RawSignalInput {
  trendGrowth: number;      // 0–30
  crossPlatform: number;    // 0–20
  viewsPerHour: number;     // 0–15
  engagement: number;       // 0–10
  searchInterest: number;   // 0–15
  nicheMatch: number;       // 0–10
  lowCompBonus: number;     // 0–8
  saturationPenalty: number; // 0–10 (subtracted)
  riskRaw: number;          // 0–100
}

export interface ScoredTopic {
  finalScore: number;
  riskLevel: RiskLevel;
  riskPenalty: number;
}

export function calcScore(input: RawSignalInput): ScoredTopic {
  const riskPenalty = input.riskRaw >= 55 ? 8 : input.riskRaw >= 30 ? 4 : 0;
  const finalScore = Math.max(
    0,
    Math.min(
      100,
      input.trendGrowth +
        input.crossPlatform +
        input.viewsPerHour +
        input.engagement +
        input.searchInterest +
        input.nicheMatch +
        input.lowCompBonus -
        input.saturationPenalty -
        riskPenalty,
    ),
  );

  const riskLevel: RiskLevel =
    input.riskRaw >= 55 ? 'High' : input.riskRaw >= 30 ? 'Med' : 'Low';

  return { finalScore, riskLevel, riskPenalty };
}

export function riskMeta(riskScore: number): { color: string; bg: string; label: RiskLevel } {
  if (riskScore >= 55) return { color: '#e8736b', bg: 'rgba(232,115,107,.14)', label: 'High' };
  if (riskScore >= 30) return { color: '#e3b54e', bg: 'rgba(227,181,78,.14)', label: 'Med' };
  return { color: '#5fd39a', bg: 'rgba(95,211,154,.12)', label: 'Low' };
}

export function trendColor(score: number): string {
  if (score >= 85) return '#5fd39a';
  if (score >= 70) return '#9fd16a';
  return '#9b9fa8';
}

export function sparkPoints(data: number[]): string {
  const max = Math.max(...data);
  const min = Math.min(...data);
  const range = max - min || 1;
  const n = data.length;
  return data
    .map((v, i) => {
      const x = (i / (n - 1)) * 100;
      const y = 28 - ((v - min) / range) * 26 - 1;
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(' ');
}
