// 화면 테마 (light/dark). 브라우저 localStorage에만 저장되는 프론트 설정.
// index.css의 :root[data-theme="light"] 토큰 오버라이드와 짝을 이룬다.
export type Theme = "light" | "dark";

const STORAGE_KEY = "kubesentinel-theme";

export function getTheme(): Theme {
  try {
    return localStorage.getItem(STORAGE_KEY) === "light" ? "light" : "dark";
  } catch {
    return "dark";
  }
}

export function applyTheme(t: Theme) {
  document.documentElement.dataset.theme = t;
  try {
    localStorage.setItem(STORAGE_KEY, t);
  } catch {
    // localStorage 불가 환경(시크릿 모드 등)에서는 현재 세션에만 적용
  }
}

// 렌더 전에 호출해 저장된 테마를 즉시 반영한다(깜빡임 방지).
export function initTheme() {
  document.documentElement.dataset.theme = getTheme();
}
