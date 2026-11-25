import { type FormEvent } from "react";
import { Select } from "./Select";
import type { Key } from "./i18n";
import {
  searchTypes,
  searchTypeLabels,
  searchPlaceholders,
  type SearchType,
} from "./search";
export function SearchForm({
  query,
  type,
  loading,
  t,
  onQuery,
  onType,
  onSubmit,
}: {
  query: string;
  type: SearchType;
  loading: boolean;
  t: (key: Key) => string;
  onQuery: (value: string) => void;
  onType: (value: SearchType) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}) {
  return (
    <form className="search" onSubmit={onSubmit}>
      <div className="search-filter">
        <Select
          label={t("searchTypeLabel")}
          value={type}
          disabled={loading}
          onChange={(value) => onType(value as SearchType)}
          options={searchTypes.map((value) => ({
            value,
            label: t(searchTypeLabels[value]),
          }))}
        />
      </div>
      <input
        value={query}
        onChange={(event) => onQuery(event.target.value)}
        aria-label={t("search")}
        placeholder={t(searchPlaceholders[type])}
        autoComplete="off"
        spellCheck={false}
        disabled={loading}
      />
      <button
        className="primary"
        type="submit"
        disabled={loading}
        aria-busy={loading}
      >
        {t(loading ? "searching" : "search")}
      </button>
    </form>
  );
}
