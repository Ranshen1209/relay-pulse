import type { Annotation, SponsorLevel, ProcessedMonitorData } from '../../../types';

/**
 * 赞助等级权重（用于置顶排序比较）
 */
export const SPONSOR_WEIGHTS: Record<SponsorLevel, number> = {
  core: 100,
  backbone: 80,
  beacon: 60,
  pulse: 40,
  signal: 20,
  public: 10,
};

/**
 * 前端层面隐藏的注解 ID 集合
 *
 * Sakrylle 主题不展示这几类注解徽章：
 * - 监测频率、公益站、赞助等级、Key Type
 * 后端依然会派生它们，但渲染层一律过滤掉。
 */
export const HIDDEN_ANNOTATION_IDS: ReadonlySet<string> = new Set([
  'monitor_frequency',
  'public_service',
  'key_type',
  'sponsor_public',
  'sponsor_signal',
  'sponsor_pulse',
  'sponsor_beacon',
  'sponsor_backbone',
  'sponsor_core',
]);

/**
 * 过滤掉前端不展示的注解
 */
export function filterVisibleAnnotations(annotations?: Annotation[]): Annotation[] {
  if (!annotations || annotations.length === 0) return [];
  return annotations.filter((ann) => !HIDDEN_ANNOTATION_IDS.has(ann.id));
}

/**
 * 检查监控项是否有任何注解（用于条件渲染）
 */
export function hasAnyAnnotation(
  item: ProcessedMonitorData,
  options: { enableAnnotations?: boolean } = {}
): boolean {
  const { enableAnnotations = true } = options;
  if (!enableAnnotations) return false;
  return filterVisibleAnnotations(item.annotations).length > 0;
}

/**
 * 检查数据列表中是否有任何项包含注解（用于显示注解列）
 */
export function hasAnyAnnotationInList(
  data: ProcessedMonitorData[],
  options: { enableAnnotations?: boolean } = {}
): boolean {
  return data.some(item => hasAnyAnnotation(item, options));
}
