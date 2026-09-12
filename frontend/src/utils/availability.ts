import type { StatusCounts } from '../types';

interface AvailabilityPoint {
  availability: number;
  statusCounts?: Partial<Pick<StatusCounts, 'available' | 'degraded' | 'unavailable' | 'missing'>>;
}

/** 按实际探测次数汇总后端得分；空时间块跳过，旧接口缺少计数时按一个样本兼容。 */
export function calculateWeightedAvailability(points: AvailabilityPoint[]): number {
  let weightedSum = 0;
  let total = 0;
  for (const point of points) {
    if (point.availability < 0) continue;
    const counts = point.statusCounts;
    const samples = counts
      ? (counts.available ?? 0) + (counts.degraded ?? 0)
        + (counts.unavailable ?? 0) + (counts.missing ?? 0)
      : 0;
    const weight = samples > 0 ? samples : 1;
    weightedSum += point.availability * weight;
    total += weight;
  }
  return total > 0 ? weightedSum / total : -1;
}
