import { expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { TermHelp } from "./TermHelp";
import { dictionaries } from "./i18n";
it("provides translated explanations while leaving unknown terms undecorated", () => {
  for (const dictionary of Object.values(dictionaries)) {
    expect(dictionary.termHelp_recognition).toBeTruthy();
    expect(dictionary.termHelp_noteCommitmentLabel).toBeTruthy();
    expect(dictionary.termHelp_fundingSubmitter).toBeTruthy();
    const html = renderToStaticMarkup(
      <TermHelp
        term="recognition"
        label={dictionary.recognition}
        t={(k) => dictionary[k]}
      />,
    );
    expect(html).toContain('type="button"');
    expect(html).toContain('aria-expanded="false"');
  }
  expect(dictionaries["zh-Hans"].termHelp_recognition).not.toBe(dictionaries.en.termHelp_recognition);
  expect(dictionaries["zh-Hant"].termHelp_recognition).not.toBe(dictionaries["zh-Hans"].termHelp_recognition);
  expect(
    renderToStaticMarkup(
      <TermHelp term="unknown" label="unknown" t={(k) => dictionaries.en[k]} />,
    ),
  ).toBe("");
});
