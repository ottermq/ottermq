export function resolveThemeColors(varNames: string[]): string[] {
	const styles = getComputedStyle(document.documentElement);
	return varNames.map((name) => styles.getPropertyValue(name).trim());
}
