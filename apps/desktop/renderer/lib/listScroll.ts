export function listRowScrollTop(input: {
  containerScrollTop: number;
  containerTop: number;
  rowTop: number;
  toolbarHeight?: number;
}): number {
  return Math.max(
    0,
    input.containerScrollTop + input.rowTop - input.containerTop - (input.toolbarHeight ?? 0),
  );
}

export function scrollListRowToTop(
  container: HTMLElement | null,
  rowKey: string | null,
  behavior: ScrollBehavior = "smooth",
): boolean {
  if (!container || !rowKey) return false;
  const row = Array.from(container.querySelectorAll<HTMLElement>("[data-list-row-key]"))
    .find((element) => element.dataset.listRowKey === rowKey);
  if (!row) return false;
  const toolbar = container.querySelector<HTMLElement>(".article-list-toolbar");
  container.scrollTo({
    top: listRowScrollTop({
      containerScrollTop: container.scrollTop,
      containerTop: container.getBoundingClientRect().top,
      rowTop: row.getBoundingClientRect().top,
      toolbarHeight: toolbar?.getBoundingClientRect().height ?? 0,
    }),
    behavior,
  });
  return true;
}
