import { Github } from 'lucide-react';
import { useTranslation } from 'react-i18next';

export function Footer() {
  const { t } = useTranslation();

  return (
    <footer className="mt-4 flex justify-end">
      <a
        href="https://github.com/Ranshen1209/relay-pulse"
        target="_blank"
        rel="noopener noreferrer"
        aria-label={t('footer.githubAria', 'GitHub')}
        title="GitHub"
        className="inline-flex items-center justify-center w-9 h-9 rounded-lg bg-elevated/50 text-secondary hover:text-accent hover:bg-muted/50 transition focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none"
      >
        <Github size={18} />
      </a>
    </footer>
  );
}
