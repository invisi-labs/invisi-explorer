import { expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import { OperationNoteInputs } from "./OperationNoteInputs";
import { AddressReference } from "./AddressReference";
import { dictionaries } from "./i18n";
const t = (key: keyof typeof dictionaries.en) => dictionaries.en[key];
it("renders each valid input as a table row with exact copying and scoped detail navigation", () => {
  const values = ["a", "b", "c"].map((c) => "0x" + c.repeat(64));
  const html = renderToStaticMarkup(
    <MemoryRouter>
      <OperationNoteInputs
        operation={{
          canonical: true,
          success: true,
          noteNullifiers: [...values, "invalid", "0x" + "0".repeat(64)],
        }}
        t={t}
        reference={(value) => (
          <AddressReference
            value={value}
            internalHref={`/nullifiers/${value}?chainId=97&protocolAddress=0x${"1".repeat(40)}&namespace=note`}
            t={t}
          />
        )}
      />
    </MemoryRouter>,
  );
  expect(html).toContain("<table");
  expect(html).toContain("Input nullifiers (3)");
  expect(html.match(/<td data-label="Nullifier"/g)).toHaveLength(3);
  expect(html.match(/aria-label="Copy full value"/g)).toHaveLength(3);
  for (const value of values)
    expect(html).toContain(
      `/nullifiers/${value}?chainId=97&amp;protocolAddress=0x${"1".repeat(40)}&amp;namespace=note`,
    );
  expect(html).not.toContain("<li");
});
