// Dark mode follows the desktop: shadcn's dark styles hang off a "dark" class on <html>,
// set here from the system preference and updated when it changes.
export function followSystemTheme(): void {
  const query = window.matchMedia("(prefers-color-scheme: dark)");
  const apply = () => document.documentElement.classList.toggle("dark", query.matches);
  apply();
  query.addEventListener("change", apply);
}
