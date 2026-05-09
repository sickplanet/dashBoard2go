async function fetchStats() {
    try {
        // Mock API hit for server stats
        // const res = await fetch('/api/v1/admin/stats');
        // const data = await res.json();
        document.getElementById('cpuLoad').innerText = Math.floor(Math.random() * 20 + 5) + "%";
        document.getElementById('ramUsage').innerText = "1.2 / 4.0 GB";
        document.getElementById('diskSpace').innerText = "45% Free";
        document.getElementById('sysInfo').innerText = "OS: Debian 13 (Bookworm)\nKernel: 6.1.0-9-amd64\nUptime: 14 days, 3 hours\nArchitecture: x86_64";
    } catch (e) { }
}

async function fetchUsers() {
    try {
        const users = await apiFetch('/api/v1/admin/users');
        let html = '';
        users.forEach(u => {
            html += `<tr>
                        <td>${u.id}</td>
                        <td>${u.username}</td>
                        <td>${u.is_admin ? '<span class="badge bg-danger">Admin</span>' : '<span class="badge bg-secondary">User</span>'}</td>
                        <td>
                            ${!u.is_admin ? `<button class="btn btn-sm btn-warning" onclick="loginAs('${u.username}')">Login as User</button>` : ''}
                        </td>
                    </tr>`;
        });
        document.getElementById('usersTable').innerHTML = html;
    } catch (e) { }
}

async function loginAs(username) {
    alert(`Impersonating ${username}. Redirecting to User Dashboard...`);
    // Store admin state so we can return
    localStorage.setItem('adminReturn', 'true');
    window.location.href = `/api/v1/auth/impersonate?user=${username}`;
}

async function fetchFirewall() {
    try {
        const data = await apiFetch('/api/v1/admin/firewall');
        const sysRules = data.rules || [];
        const sqlRules = data.sqlRules || [];
        const fwStatus = data.status || "active";

        const badge = document.getElementById('fwStatusBadge');
        document.getElementById('fwInstallUfwBtn').classList.add('d-none');
        document.getElementById('fwInstallNftBtn').classList.add('d-none');
        document.getElementById('fwEnableBtn').classList.add('d-none');
        document.getElementById('fwDisableBtn').classList.add('d-none');

        if (fwStatus === "not_installed") {
            badge.className = "badge bg-dark ms-2";
            badge.innerText = "Not Installed";
            document.getElementById('fwInstallUfwBtn').classList.remove('d-none');
            document.getElementById('fwInstallNftBtn').classList.remove('d-none');
        } else if (fwStatus === "inactive") {
            badge.className = "badge bg-warning text-dark ms-2";
            badge.innerText = "Inactive";
            document.getElementById('fwEnableBtn').classList.remove('d-none');
        } else {
            badge.className = "badge bg-success ms-2";
            badge.innerText = "Active";
            document.getElementById('fwDisableBtn').classList.remove('d-none');
        }

        const allRules = [];

        // Match SQL
        sqlRules.forEach(sq => {
            let match = sysRules.find(sy => sy.port == sq.port && (sy.protocol || "").toLowerCase() === (sq.protocol || "").toLowerCase() && (sy.action || "").toUpperCase() === (sq.action || "").toUpperCase());
            if (match) {
                allRules.push({ ...sq, status: 'OK' });
            } else {
                allRules.push({ ...sq, status: 'MISSING_IN_OS' });
            }
        });

        // Match OS
        sysRules.forEach(sy => {
            let match = sqlRules.find(sq => sy.port == sq.port && (sy.protocol || "").toLowerCase() === (sq.protocol || "").toLowerCase() && (sy.action || "").toUpperCase() === (sq.action || "").toUpperCase());
            if (!match) {
                allRules.push({ ...sy, status: 'ROGUE_OS' });
            }
        });

        let html = '';
        if (allRules && allRules.length > 0) {
            allRules.forEach(r => {
                let actionClass = (r.action || r.Action || "").toUpperCase().includes('ALLOW') ? 'bg-success' : 'bg-danger';

                let statusBadge = '<span class="badge bg-success">Active/Synced</span>';
                let actionBtn = '';
                if (r.status === 'MISSING_IN_OS') {
                    statusBadge = '<span class="badge bg-warning text-dark">Missing from UFW</span>';
                    actionBtn = `<button class="btn btn-sm btn-primary m-1" onclick="alert('TODO: Implement push to UFW UI')">Reapply</button>`;
                } else if (r.status === 'ROGUE_OS') {
                    statusBadge = '<span class="badge bg-danger">Rogue UFW Rule</span>';
                    actionBtn = `<button class="btn btn-sm btn-warning m-1" onclick="alert('TODO: Implement save to DB UI')">Save to DB</button>
                                         <button class="btn btn-sm btn-danger m-1" onclick="alert('TODO: Implement Delete UFW UI')">Delete OS Rule</button>`;
                }

                html += `<tr class="${r.status === 'MISSING_IN_OS' ? 'table-warning' : (r.status === 'ROGUE_OS' ? 'table-danger' : '')}">
                            <td>${r.port || r.Port || '*'}${r.protocol || r.Protocol ? '/' + (r.protocol || r.Protocol) : ''}</td>
                            <td><span class="badge ${actionClass}">${r.action || r.Action}</span></td>
                            <td>${r.source || r.Source || 'Anywhere'} ${r.comment || r.Comment ? ' - ' + (r.comment || r.Comment) : ''}</td>
                            <td>${statusBadge}</td>
                            <td>${actionBtn}</td>
                        </tr>`;
            });
        } else {
            html = '<tr><td colspan="5">No rules found or UFW disabled.</td></tr>';
        }
        document.getElementById('fwTable').innerHTML = html;
    } catch (e) {
        document.getElementById('fwTable').innerHTML = '<tr><td colspan="5">Failed to load firewall rules.</td></tr>';
    }
}

async function installFirewall(backend) {
    if (!confirm(`Install ${backend} on server?`)) return;
    await apiFetch('/api/v1/admin/firewall/install', 'POST', { backend });
    alert('A backend installation job has executed. Please refresh shortly.');
    location.reload();
}

async function toggleFirewall(action) {
    await apiFetch('/api/v1/admin/firewall/toggle', 'PUT', { action });
    location.reload();
}


async function fetchServices() {
    try {
        if (!document.getElementById('servicesStatus')) return;
        const data = await apiFetch('/api/v1/admin/services');
        const container = document.getElementById('servicesStatus');
        if (!container) return;
        container.innerHTML = '';

        data.forEach(service => {
            const badgeClass = service.active ? 'bg-success' : 'bg-danger';
            const badge = `<span class="badge ${badgeClass} fs-6">${service.service.toUpperCase()} - ${service.status}</span>`;
            container.innerHTML += badge;
        });
    } catch (e) {
        const cErr = document.getElementById('servicesStatus'); if (cErr) { cErr.innerHTML = '<span class="badge bg-danger">Network Error</span>'; }
    }
}

function showUpdateScreen() {
    document.body.innerHTML = `
            <div class="bg-dark text-white vh-100 d-flex flex-column justify-content-center align-items-center">
                <h2><i class="bi bi-cloud-arrow-down spin text-warning"></i> Updating dashBoard2go...</h2>
                <p id="updateStatusText" class="text-secondary mt-2">Do not close this window. Services are currently down.</p>
                <textarea id="updateLogBox" class="form-control bg-dark border-secondary text-success mt-3 font-monospace" readonly></textarea>
                <button id="updateReloadBtn" class="btn btn-success mt-4 d-none fw-bold px-5" onclick="location.reload()">Update Complete - Reload Dashboard</button>
            </div>`;
}

async function monitorUpdate() {
    const logBox = document.getElementById('updateLogBox');
    let isComplete = false;

    // The public raw log URL in the default debian directory where Nginx runs
    const publicLogUrl = window.location.protocol + "//" + window.location.hostname + "/dashboard2go_update.log";

    const poll = setInterval(async () => {
        // 1. Try fetching from public Nginx/Apache folder (graceful fail if dead)
        try {
            const pubRes = await fetch(publicLogUrl, { cache: 'no-store' });
            if (pubRes.ok) {
                const text = await pubRes.text();
                if (text && text.length > 0) logBox.value = text;
                logBox.scrollTop = logBox.scrollHeight;
            }
        } catch (e) { }

        // 2. Poll native backend API to see if systemctl restarted the daemon
        try {
            const backRes = await fetch('/api/v1/admin/updates/log', { cache: 'no-store' });
            if (backRes.ok) {
                const backData = await backRes.json();
                if (backData.log) {
                    logBox.value = backData.log;
                    logBox.scrollTop = logBox.scrollHeight;
                }
                if (backData.log.includes("Update completed successfully")) {
                    clearInterval(poll);
                    document.getElementById('updateReloadBtn').classList.remove('d-none');
                    document.querySelector('h2').innerHTML = '<i class="bi bi-check-circle-fill text-success"></i> Update Finished';
                    const statusText = document.getElementById('updateStatusText');
                    if (statusText) statusText.innerText = 'Updated sucessfully! You can close this window or go back to admin dashboard';
                }
            }
        } catch (e) { }
    }, 1500);
}

async function checkForUpdates() {
    try {
        const data = await apiFetch('/api/v1/admin/updates/check', 'POST');
        if (data.has_update && data.release) {
            document.getElementById('checkUpdateBtn').classList.add('d-none');
            document.getElementById('updateVersionText').innerText = data.release.name || data.release.tag_name;
            document.getElementById('updateDateText').innerText = "Published: " + new Date(data.release.published_at).toLocaleString();
            document.getElementById('updateBodyText').innerText = data.release.body || "No changelog provided.";
            document.getElementById('updateInfoDiv').classList.remove('d-none');
        } else if (data.has_update) {
            // Fallback if release API struct failed but update exists
            document.getElementById('checkUpdateBtn').classList.add('d-none');
            document.getElementById('updateVersionText').innerText = data.version;
            document.getElementById('updateDateText').innerText = "New release found in DB cache.";
            document.getElementById('updateBodyText').innerText = "Available version: " + data.version;
            document.getElementById('updateInfoDiv').classList.remove('d-none');
        } else {
            alert('You are already on the latest version.');
        }
    } catch (e) {
        alert('Network error passing check payload signal.');
    }
}

async function applyUpdate() {
    if (!confirm("Are you sure you want to construct the firmware updates?\nServices will go offline and you will be transitioned to the Live Monitoring view.")) return;
    try {
        const data = await apiFetch('/api/v1/admin/updates/apply', 'POST');
        if (data.status === "ok") {
            showUpdateScreen();
            monitorUpdate();
        } else {
            alert('Backend rejected upgrade signal.');
        }
    } catch (e) {
        alert('Network error passing update payload signal.');
    }
}

async function fetchUpdates() {
    if (!document.getElementById('updateMenu')) return;
    try {
        const data = await apiFetch('/api/v1/admin/updates');
        if (data.update_available && data.update_available !== "" && data.update_available !== "false") {
            document.getElementById('updateMenu').classList.remove('d-none');
            document.getElementById('updateBadge').innerText = data.update_available;
        }
    } catch (e) { }
}

setInterval(fetchStats, 5000);
setInterval(fetchServices, 10000);
setInterval(fetchUpdates, 30000); // Check updates every 30 seconds
fetchStats();
fetchUsers();
fetchUpdates();
fetchServices();
fetchFirewall();
