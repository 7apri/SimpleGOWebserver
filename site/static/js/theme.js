/**
 * Theme Management Utility.
 * Handles light/dark mode switching, LocalStorage persistence, 
 * and synchronization with browser meta tags and CSS custom properties.
 * @module ThemeService
 */

/** @constant {string} Key for theme storage in LocalStorage */
const THEME_KEY = 'user-theme';

/** @type {HTMLMetaElement|null} Reference to the <meta name="theme-color"> tag */
let metaEL

/** @constant {HTMLElement} The root document.getElementById("langSwitch-btn")<html> element of the document */
const root = document.documentElement;

class Theme {
  constructor({ ariaLabel, iconId, cssName, metaColor }) {
    this.ariaLabel = ariaLabel;
    this.iconId = iconId;
    this.cssName = cssName;
    this.metaColor = metaColor;
  }

  applyTheme(metaEl) {
    root.setAttribute('data-theme', this.cssName);
    localStorage.setItem(THEME_KEY, this.cssName);
    if (metaEl) metaEl.setAttribute('content', this.metaColor);
  }
}

const themes = [
  new Theme({
    ariaLabel: window.tr("switch_dark"),
    iconId:    "lightMode",
    cssName:   "light",
    metaColor: "#fff",
  }),
  new Theme({
    ariaLabel: window.tr("switch_light"),
    iconId:    "darkMode",
    cssName:   "dark",
    metaColor: "#000",
  })
];
let currentThemeIndex = 0;

function updateTheme(themeSwitchBtn, btnImg, metaEl) {
  const theme = themes[currentThemeIndex];
  const nextTheme = themes[(currentThemeIndex + 1) % themes.length];

  theme.applyTheme(metaEl);

  if (btnImg) {
    btnImg.setAttribute('href', `/static/assets/iconBundle.svg#${theme.iconId}`);
  }
  if (themeSwitchBtn) {
    themeSwitchBtn.setAttribute('aria-label', nextTheme.ariaLabel);
  }
}

function setupTheme() {
  const metaEl = document.querySelector('meta[name="theme-color"]');
  const themeSwitchBtn = document.getElementById('themeSwitch-btn');
  const btnImg = themeSwitchBtn?.querySelector('use');

  const savedTheme = localStorage.getItem(THEME_KEY) || root.getAttribute('data-theme');
  currentThemeIndex = Math.max(0, themes.findIndex(t => t.cssName === savedTheme));

  updateTheme(themeSwitchBtn, btnImg, metaEl);

  themeSwitchBtn?.addEventListener('click', () => {
    currentThemeIndex = (currentThemeIndex + 1) % themes.length;
    updateTheme(themeSwitchBtn, btnImg, metaEl);
  });

  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', e => {
    currentThemeIndex = (currentThemeIndex + 1) % themes.length;
    updateTheme(themeSwitchBtn, btnImg, metaEl);
  });
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', setupTheme);
} else {
  setupTheme();
}
