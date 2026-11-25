import type { Key } from "./i18n";
export function PageControls({
  page,
  total,
  loading,
  error,
  hasNext,
  previous,
  next,
  t,
}: {
  page: number;
  total?: string;
  loading: boolean;
  error?: unknown;
  hasNext: boolean;
  previous: () => void;
  next: () => void;
  t: (key: Key) => string;
}) {
  return (
    <div className="pagination">
      <span>
        {total === undefined
          ? t("paginationPage").replace("{page}", String(page))
          : t("paginationSummary")
              .replace("{total}", total)
              .replace("{page}", String(page))}
      </span>
      <div>
        <button disabled={loading || page === 1} onClick={previous}>
          {t("previous")}
        </button>
        <button disabled={loading || !!error || !hasNext} onClick={next}>
          {t("next")}
        </button>
      </div>
    </div>
  );
}
