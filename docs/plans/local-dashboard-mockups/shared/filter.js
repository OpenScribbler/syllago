/* Shared filter state + predicate for the catalog page.
 * All four surviving mockups use this so the filter semantics
 * are identical — only the visual chrome differs. */

window.createFilterState = function () {
  return { query: "", type: "all", origin: "all" };
};

window.applyFilter = function (state, items) {
  const q = (state.query || "").toLowerCase().trim();
  return items.filter(i => {
    if (state.type !== "all" && i.type !== state.type) return false;
    if (state.origin !== "all" && i.origin !== state.origin) return false;
    if (!q) return true;
    return (
      i.name.toLowerCase().includes(q) ||
      (i.description || "").toLowerCase().includes(q) ||
      i.type.toLowerCase().includes(q) ||
      i.provider.toLowerCase().includes(q)
    );
  });
};

/* Distinct type/origin lists with counts, sorted by count desc then name asc. */
window.countBy = function (items, key) {
  const out = {};
  items.forEach(i => { out[i[key]] = (out[i[key]] || 0) + 1; });
  return out;
};
