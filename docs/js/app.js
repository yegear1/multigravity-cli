/**
 * Multigravity CLI — Interactive Showcase & Onboarding
 * Pure vanilla JavaScript, zero runtime dependencies.
 */

(function () {
  'use strict';

  // --- Theme Toggle ---
  const themeToggle = document.getElementById('theme-toggle');
  const themeIcon = document.getElementById('theme-icon');

  function initTheme() {
    const savedTheme = localStorage.getItem('mg-theme');
    const systemPrefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    const theme = savedTheme || (systemPrefersDark ? 'dark' : 'light');
    setTheme(theme);
  }

  function setTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('mg-theme', theme);
    if (themeIcon) {
      themeIcon.textContent = theme === 'dark' ? '☀️' : '🌙';
    }
  }

  if (themeToggle) {
    themeToggle.addEventListener('click', () => {
      const current = document.documentElement.getAttribute('data-theme') || 'dark';
      setTheme(current === 'dark' ? 'light' : 'dark');
    });
  }

  // --- Copy to Clipboard & Toast ---
  const toastContainer = document.getElementById('toast-container');
  let toastTimeout;

  function showToast(message) {
    if (!toastContainer) return;
    toastContainer.textContent = message;
    toastContainer.classList.add('show');
    clearTimeout(toastTimeout);
    toastTimeout = setTimeout(() => {
      toastContainer.classList.remove('show');
    }, 2200);
  }

  function setupCopyButtons() {
    document.addEventListener('click', async (e) => {
      const btn = e.target.closest('.copy-btn, .step-copy-btn, .cmd-copy-btn');
      if (!btn) return;

      const text = btn.getAttribute('data-copy') || btn.dataset.code;
      if (!text) return;

      try {
        await navigator.clipboard.writeText(text);
        const originalText = btn.innerHTML;
        btn.classList.add('copied');
        btn.innerHTML = '✓ Copied';
        showToast(`Copied to clipboard: "${text.substring(0, 32)}${text.length > 32 ? '...' : ''}"`);
        setTimeout(() => {
          btn.classList.remove('copied');
          btn.innerHTML = originalText;
        }, 1800);
      } catch (err) {
        // Fallback for older browsers
        const textarea = document.createElement('textarea');
        textarea.value = text;
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
        showToast('Copied to clipboard!');
      }
    });
  }

  // --- Interactive Terminal Simulation ---
  const terminalScenarios = {
    tui: {
      cmd: 'multigravity',
      output: `\n\x1b[36m┌─────────────────────────────────────────────────────────────┐\x1b[0m
\x1b[36m│\x1b[0m  \x1b[1m\x1b[35mMULTIGRAVITY PROFILES\x1b[0m                                      \x1b[36m│\x1b[0m
\x1b[36m└─────────────────────────────────────────────────────────────┘\x1b[0m

  \x1b[1m[1]\x1b[0m  \x1b[32m●\x1b[0m work           \x1b[32mrunning\x1b[0m   \x1b[90m[shared]\x1b[0m     \x1b[34m[color: blue]\x1b[0m
  \x1b[1m[2]\x1b[0m  \x1b[90m○\x1b[0m personal       \x1b[90midle\x1b[0m      \x1b[36m[auth-only]\x1b[0m  \x1b[32m[color: emerald]\x1b[0m
  \x1b[1m[3]\x1b[0m  \x1b[90m○\x1b[0m client-acme    \x1b[90midle\x1b[0m      \x1b[90m[isolated]\x1b[0m   \x1b[33m[color: amber]\x1b[0m

  \x1b[1m[n]\x1b[0m  Create new profile
  \x1b[1m[q]\x1b[0m  Quit

\x1b[36mSelect profile [1-3], 'n' for new, 'q' to quit:\x1b[0m 2
\x1b[90mLaunching profile "personal" (auth-only) with custom emerald theme...\x1b[0m
\x1b[32m✔ Antigravity IDE instance launched successfully (PID 84210)\x1b[0m`
    },
    auth_only: {
      cmd: 'multigravity new dev --auth-only --color blue',
      output: `\n\x1b[90mValidating profile name "dev"...\x1b[0m
\x1b[32m✔ Layout created at ~/AntigravityProfiles/dev\x1b[0m
\x1b[90mApplying Auth-Only configuration:\x1b[0m
  \x1b[36m➜\x1b[0m Linking host extensions (~/.antigravity/extensions)... \x1b[32mdone\x1b[0m
  \x1b[36m➜\x1b[0m Linking host settings.json, keybindings and snippets... \x1b[32mdone\x1b[0m
  \x1b[36m➜\x1b[0m Symlinking host .gitconfig and SSH credentials... \x1b[32mdone\x1b[0m
  \x1b[36m➜\x1b[0m Applying profile color preset "blue" (#1e3a8a)... \x1b[32mdone\x1b[0m
  \x1b[36m➜\x1b[0m Generating desktop launcher with embedded icon... \x1b[32mdone\x1b[0m

\x1b[1m\x1b[32m✔ Profile "dev" created successfully!\x1b[0m
\x1b[90mInitial disk footprint: \x1b[1m\x1b[32m1.8 MB\x1b[0m \x1b[90m(saved ~498 MB vs full profile isolation)\x1b[0m
\x1b[90mRun \x1b[36mmultigravity dev\x1b[0m \x1b[90mor click the desktop shortcut to start.\x1b[0m`
    },
    quota: {
      cmd: 'multigravity quota dev',
      output: `\n\x1b[1m\x1b[35mAI Quota & Token Telemetry for profile "dev":\x1b[0m

  \x1b[1m\x1b[36mGoogle Gemini Quotas:\x1b[0m
    gemini-5h       \x1b[32m[████████████████░░░░]\x1b[0m  \x1b[1m82.4%\x1b[0m  \x1b[90m(resets in 2h 11m)\x1b[0m
    gemini-weekly   \x1b[32m[████████████████████]\x1b[0m \x1b[1m100.0%\x1b[0m  \x1b[90m(resets in 5d 14h)\x1b[0m

  \x1b[1m\x1b[34m3rd Party Models (Claude 3.7 / GPT):\x1b[0m
    3p-5h           \x1b[33m[██████████████░░░░░░]\x1b[0m  \x1b[1m70.0%\x1b[0m  \x1b[90m(resets in 3h 48m)\x1b[0m
    3p-weekly       \x1b[33m[████████████░░░░░░░░]\x1b[0m  \x1b[1m61.2%\x1b[0m  \x1b[90m(resets in 2d 19h)\x1b[0m

\x1b[90m─────────────────────────────────────────────────────────────\x1b[0m
  \x1b[1mWatchdog Prime Status:\x1b[0m \x1b[32m● Active\x1b[0m
  \x1b[90mHeadless server: Ephemeral RPC check completed in 0.38s\x1b[0m
  \x1b[90mNext scheduled cycle prime: in 2d 19h (jitter: 14m)\x1b[0m`
    },
    clean: {
      cmd: 'multigravity clean --all',
      output: `\n\x1b[90mScanning 4 profile(s) for volatile Chromium/Electron caches...\x1b[0m

  \x1b[1m[1/4] work:\x1b[0m
    \x1b[33m⚠ Profile is running (skipped to protect active SQLite DBs)\x1b[0m
  \x1b[1m[2/4] personal:\x1b[0m
    \x1b[90mRemoving GPUCache, Code Cache, DawnGraphiteCache, Crashpad...\x1b[0m
    \x1b[32m✔ Cleaned 412 MB\x1b[0m
  \x1b[1m[3/4] dev:\x1b[0m
    \x1b[90mRemoving Service Worker, cache storage, logs...\x1b[0m
    \x1b[32m✔ Cleaned 185 MB\x1b[0m
  \x1b[1m[4/4] client-acme:\x1b[0m
    \x1b[90mRemoving old caches and crash reports...\x1b[0m
    \x1b[32m✔ Cleaned 860 MB\x1b[0m

\x1b[1m\x1b[32m✔ Cache cleanup complete! Total reclaimed disk space: 1.45 GB\x1b[0m`
    }
  };

  const termCmdElem = document.getElementById('term-cmd-text');
  const termOutputElem = document.getElementById('term-output-text');
  const termTabButtons = document.querySelectorAll('.term-tab-btn');
  const termCopyCmdBtn = document.getElementById('term-copy-cmd-btn');
  let currentScenarioKey = 'tui';
  let typingTimer;

  // Convert simple ANSI color codes to styled HTML spans
  function ansiToHtml(text) {
    const ansiMap = [
      { regex: /\x1b\[32m/g, class: 't-green' },
      { regex: /\x1b\[36m/g, class: 't-cyan' },
      { regex: /\x1b\[33m/g, class: 't-yellow' },
      { regex: /\x1b\[34m/g, class: 't-blue' },
      { regex: /\x1b\[35m/g, class: 't-purple' },
      { regex: /\x1b\[90m/g, class: 't-gray' },
      { regex: /\x1b\[1m/g, class: 't-bold' },
      { regex: /\x1b\[0m/g, close: true }
    ];

    let html = '';
    let openSpans = 0;
    let i = 0;

    while (i < text.length) {
      if (text.substr(i, 2) === '\x1b[') {
        const end = text.indexOf('m', i);
        if (end !== -1) {
          const code = text.substring(i, end + 1);
          i = end + 1;
          if (code === '\x1b[0m') {
            while (openSpans > 0) {
              html += '</span>';
              openSpans--;
            }
          } else {
            const match = ansiMap.find(m => m.regex.test(code));
            if (match && match.class) {
              html += `<span class="${match.class}">`;
              openSpans++;
            }
          }
          continue;
        }
      }
      const char = text[i];
      if (char === '<') html += '&lt;';
      else if (char === '>') html += '&gt;';
      else if (char === '&') html += '&amp;';
      else html += char;
      i++;
    }

    while (openSpans > 0) {
      html += '</span>';
      openSpans--;
    }

    return html;
  }

  function playTerminalScenario(key) {
    currentScenarioKey = key;
    const scenario = terminalScenarios[key];
    if (!scenario || !termCmdElem || !termOutputElem) return;

    clearTimeout(typingTimer);
    termCmdElem.textContent = '';
    termOutputElem.innerHTML = '';

    if (termCopyCmdBtn) {
      termCopyCmdBtn.setAttribute('data-copy', scenario.cmd);
    }

    // Type out the command character by character
    let charIndex = 0;
    const cmdText = scenario.cmd;

    function typeNextChar() {
      if (charIndex < cmdText.length) {
        termCmdElem.textContent += cmdText[charIndex];
        charIndex++;
        typingTimer = setTimeout(typeNextChar, 28);
      } else {
        // Output appears after a realistic 220ms pause
        typingTimer = setTimeout(() => {
          termOutputElem.innerHTML = ansiToHtml(scenario.output);
        }, 220);
      }
    }

    typeNextChar();
  }

  function setupTerminalTabs() {
    termTabButtons.forEach(btn => {
      btn.addEventListener('click', () => {
        const scenarioKey = btn.getAttribute('data-tab');
        if (scenarioKey === currentScenarioKey) return;

        termTabButtons.forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
        playTerminalScenario(scenarioKey);
      });
    });

    const replayBtn = document.getElementById('term-replay-btn');
    if (replayBtn) {
      replayBtn.addEventListener('click', () => {
        playTerminalScenario(currentScenarioKey);
      });
    }
  }

  // --- OS Onboarding Tabs ---
  function setupOSTabs() {
    const osButtons = document.querySelectorAll('.os-tab-btn');
    const osPanes = document.querySelectorAll('.os-content-pane');

    osButtons.forEach(btn => {
      btn.addEventListener('click', () => {
        const os = btn.getAttribute('data-os');
        osButtons.forEach(b => b.classList.remove('active'));
        osPanes.forEach(p => p.classList.remove('active'));

        btn.classList.add('active');
        const activePane = document.getElementById(`pane-${os}`);
        if (activePane) activePane.classList.add('active');
      });
    });
  }

  // --- Disk Footprint Calculator ---
  function setupCalculator() {
    const slider = document.getElementById('profile-count-slider');
    const countDisplay = document.getElementById('profile-count-val');
    const fullSizeDisplay = document.getElementById('full-profile-size');
    const authSizeDisplay = document.getElementById('auth-profile-size');
    const savedSizeDisplay = document.getElementById('saved-space-val');

    if (!slider) return;

    function updateCalculations() {
      const count = parseInt(slider.value, 10);
      if (countDisplay) countDisplay.textContent = count;

      // Full profile ~500 MB each; Auth-only profile ~2 MB each
      const fullMB = count * 500;
      const authMB = count * 2;
      const savedMB = fullMB - authMB;
      const percentSaved = ((savedMB / fullMB) * 100).toFixed(1);

      function formatMB(mb) {
        if (mb >= 1024) {
          return `${(mb / 1024).toFixed(2)} GB`;
        }
        return `${mb} MB`;
      }

      if (fullSizeDisplay) fullSizeDisplay.textContent = formatMB(fullMB);
      if (authSizeDisplay) authSizeDisplay.textContent = formatMB(authMB);
      if (savedSizeDisplay) {
        savedSizeDisplay.textContent = `${formatMB(savedMB)} (${percentSaved}% saved)`;
      }
    }

    slider.addEventListener('input', updateCalculations);
    updateCalculations();
  }

  // --- Command Explorer Filter & Search ---
  function setupCommandExplorer() {
    const searchInput = document.getElementById('cmd-search-input');
    const filterButtons = document.querySelectorAll('.filter-tag-btn');
    const cmdCards = document.querySelectorAll('.cmd-card');

    let activeFilter = 'all';
    let searchQuery = '';

    function filterCards() {
      cmdCards.forEach(card => {
        const category = card.getAttribute('data-category');
        const name = card.getAttribute('data-name').toLowerCase();
        const desc = card.querySelector('.cmd-desc')?.textContent.toLowerCase() || '';

        const matchesFilter = (activeFilter === 'all' || category === activeFilter);
        const matchesSearch = !searchQuery || name.includes(searchQuery) || desc.includes(searchQuery);

        if (matchesFilter && matchesSearch) {
          card.style.display = 'flex';
        } else {
          card.style.display = 'none';
        }
      });
    }

    if (searchInput) {
      searchInput.addEventListener('input', (e) => {
        searchQuery = e.target.value.toLowerCase().trim();
        filterCards();
      });
    }

    filterButtons.forEach(btn => {
      btn.addEventListener('click', () => {
        filterButtons.forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
        activeFilter = btn.getAttribute('data-filter');
        filterCards();
      });
    });
  }

  // --- Initialize Everything on DOM Ready ---
  document.addEventListener('DOMContentLoaded', () => {
    initTheme();
    setupCopyButtons();
    setupTerminalTabs();
    playTerminalScenario('tui');
    setupOSTabs();
    setupCalculator();
    setupCommandExplorer();
  });
})();
