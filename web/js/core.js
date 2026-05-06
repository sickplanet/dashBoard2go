// Core JavaScript for dashBoard2go
// Handles generic API integrations, global state, and Bootstrap utilities

async function apiFetch(endpoint, method = "GET", body = null) {
    try {
        const options = {
            method: method,
            headers: {}
        };
        
        if (body) {
            options.headers["Content-Type"] = "application/json";
            options.body = JSON.stringify(body);
        }
        
        const res = await fetch(endpoint, options);
        let data = null;
        
        try {
            data = await res.json();
        } catch(e) {}
        
        if (!res.ok) {
            const errMsg = (data && data.error) ? data.error : `HTTP error! status: ${res.status}`;
            throw new Error(errMsg);
        }
        return data;
    } catch (e) {
        console.error("API Call Failed:", endpoint, e);
        throw e;
    }
}

// Global functions available on ALL pages
function logout() {
    window.location.href = '/login';
}

function returnAdmin() {
    window.location.href = '/admin';
}
