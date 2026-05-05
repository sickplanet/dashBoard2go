// Theming Engine
// Fetches the user category theme on page load and manipulates the DOM correctly.
document.addEventListener("DOMContentLoaded", async () => {
    // We assume the page loads with data-bs-theme="dark" by default statically to prevent white-flashes,
    // then JavaScript overrides it depending on the user's category via API.
    
    // In a real application, the backend would template this variable in or we fetch it.
    // For this demo, we simulate fetching the assigned theme/layout for the logged-in user.
    try {
        // Stub endpoint call: /api/v1/user/me/theme
        // We will simulate the user preferring a layout dynamically.
        const storedTheme = localStorage.getItem('db2go_theme') || 'dark';
        document.documentElement.setAttribute('data-bs-theme', storedTheme);
        
        console.log(`[Theme Engine] Layout initialized with UI theme: ${storedTheme}`);
    } catch (e) {
        console.error("Theme engine failed to load preferences.", e);
    }
});

// A utility to let the Admin or User force-switch themes live
function switchTheme(mode) {
    document.documentElement.setAttribute('data-bs-theme', mode);
    localStorage.setItem('db2go_theme', mode);
}
