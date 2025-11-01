import React from 'react';
import { cn } from '../../utils/cn';

export interface InfoCardProps {
  title: string;
  children: React.ReactNode;
  className?: string;
  icon?: React.ReactNode;
  badge?: React.ReactNode;
}

export const InfoCard: React.FC<InfoCardProps> = ({
  title,
  children,
  className = '',
  icon,
  badge,
}) => {
  return (
    <div className={cn(
      'bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 hover:shadow-md transition-shadow',
      className
    )}>
      <div className="flex items-center justify-between mb-3">
        <h4 className="text-sm font-semibold text-gray-900 dark:text-gray-100 flex items-center gap-2">
          {icon}
          {title}
        </h4>
        {badge}
      </div>
      <div className="text-sm text-gray-600 dark:text-gray-400">
        {children}
      </div>
    </div>
  );
};

export interface InfoItemProps {
  label: string;
  value: React.ReactNode;
  copyable?: boolean;
  className?: string;
}

export const InfoItem: React.FC<InfoItemProps> = ({
  label,
  value,
  copyable = false,
  className = '',
}) => {
  const handleCopy = () => {
    if (typeof value === 'string') {
      navigator.clipboard.writeText(value);
    }
  };

  return (
    <div className={cn('flex justify-between items-start py-2 border-b border-gray-100 dark:border-gray-700 last:border-b-0', className)}>
      <dt className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
        {label}
      </dt>
      <dd className="text-sm text-gray-900 dark:text-gray-100 font-mono break-all max-w-[60%] flex items-center gap-2">
        {value}
        {copyable && typeof value === 'string' && (
          <button
            type="button"
            onClick={handleCopy}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
            title="复制"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
              <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
            </svg>
          </button>
        )}
      </dd>
    </div>
  );
};