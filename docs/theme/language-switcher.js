(function() {
    'use strict';

    // Detect current language from URL path
    function getCurrentLang() {
        return window.location.pathname.includes('/ko/') ? 'ko' : 'en';
    }

    // Get the other language URL
    function getOtherLangUrl() {
        const currentPath = window.location.pathname;
        const currentLang = getCurrentLang();

        if (currentLang === 'en') {
            return currentPath.replace(/^\//, '/ko/');
        } else {
            return currentPath.replace('/ko/', '/');
        }
    }

    // Create language toggle button
    function createLangToggle() {
        const button = document.createElement('button');
        button.id = 'lang-toggle';
        button.className = 'icon-button';
        button.setAttribute('aria-label', 'Switch language');
        button.setAttribute('title', 'Switch language');

        const currentLang = getCurrentLang();
        button.textContent = currentLang === 'en' ? '한국어' : 'English';

        button.addEventListener('click', function() {
            const otherLangUrl = getOtherLangUrl();
            window.location.href = otherLangUrl;
        });

        return button;
    }

    // Insert toggle button into mdBook toolbar
    function insertToggle() {
        const rightButtons = document.querySelector('.right-buttons');
        if (rightButtons) {
            const toggle = createLangToggle();
            rightButtons.insertBefore(toggle, rightButtons.firstChild);
        }
    }

    // Initialize on page load
    function init() {
        insertToggle();
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
