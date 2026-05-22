import { RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface RefreshButtonProps {
  loading: boolean;
  autoRefresh?: boolean;
  refreshCooldown: boolean;
  onRefresh: () => void;
  onToggleAutoRefresh?: () => void;
  /** 按钮尺寸：'sm' 用于移动端，'md' 用于桌面端 */
  size?: 'sm' | 'md';
}

/**
 * 合并的刷新按钮组件
 * - 点击：切换自动刷新开关 + 立即刷新（冷却期内不切换状态）
 * - 自动刷新状态通过按钮颜色表示（开启=success，关闭=muted）
 */
export function RefreshButton({
  loading,
  autoRefresh = true,
  refreshCooldown,
  onRefresh,
  onToggleAutoRefresh,
  size = 'md',
}: RefreshButtonProps) {
  const { t } = useTranslation();

  const isSmall = size === 'sm';
  const buttonSize = isSmall ? 'p-1.5' : 'p-2';
  const iconSize = isSmall ? 14 : 16;
  const minSize = isSmall ? '' : 'w-8 h-8';

  // 点击处理：切换状态 + 刷新（冷却期内不切换状态）
  const handleClick = () => {
    if (onToggleAutoRefresh && !refreshCooldown) {
      onToggleAutoRefresh();
    }
    onRefresh();
  };

  // 按钮样式：根据 autoRefresh 状态决定颜色
  const buttonStyles = onToggleAutoRefresh
    ? autoRefresh
      ? 'bg-success/10 text-success border-success/50 hover:bg-success/20'
      : 'bg-elevated/50 text-muted border-muted hover:bg-muted/30'
    : 'bg-accent/10 text-accent border-accent/20 hover:bg-accent/20';

  // 提示文案
  const buttonTitle = onToggleAutoRefresh
    ? autoRefresh
      ? t('controls.autoRefresh.enabledHint')
      : t('controls.autoRefresh.disabledHint')
    : t('common.refresh');

  return (
    <div className="relative inline-flex items-center">
      <button
        type="button"
        onClick={handleClick}
        className={`${buttonSize} rounded-lg ${buttonStyles} transition-colors border group ${minSize} flex items-center justify-center cursor-pointer focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none`}
        title={buttonTitle}
        aria-label={buttonTitle}
      >
        <RefreshCw
          size={iconSize}
          className={loading ? 'animate-spin' : ''}
        />
      </button>

      {/* 冷却提示 */}
      {refreshCooldown && (
        <div className={`absolute top-full left-1/2 -translate-x-1/2 ${isSmall ? 'mt-1' : 'mt-2'} px-2 py-1 bg-elevated text-secondary text-[10px] rounded whitespace-nowrap shadow-lg border border-default z-50`}>
          {t('common.refreshCooldown')}
        </div>
      )}
    </div>
  );
}
