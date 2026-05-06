        if (localStorage.getItem('adminReturn') === 'true') {
            document.getElementById('impersonationBar').classList.remove('d-none');
        }

        
        // --- REAL API FETCH LOGIC ---
        
        // We simulate currently logged-in user from a meta tag or local storage. 
        // For simplicity now, let's assume 'demo_user' if not tracked natively yet!
        const currentUser = "demo_user";

        async function loadDomains() {
            try {
                const data = await apiFetch('/api/v1/user/domains?username=' + currentUser);
                const tbody = document.getElementById('domainTable');
                if(data.domains && data.domains.length > 0) {
                    tbody.innerHTML = data.domains.map(d => `<tr><td>${d.domain}</td><td>/home/${currentUser}/${d.domain}</td><td>${d.php_version}</td><td>${d.ssl_enabled ? 'Yes' : 'No'}</td><td><button class='btn btn-sm btn-danger'><i class='bi bi-trash'></i></button></td></tr>`).join('');
                } else {
                    tbody.innerHTML = '<tr><td colspan="5" class="text-center text-muted">No domains configured.</td></tr>';
                }
            } catch(e) {}
        }

        async function loadDatabases() {
            try {
                const data = await apiFetch('/api/v1/user/databases?username=' + currentUser);
                const tbody = document.getElementById('dbTable');
                if(data.databases && data.databases.length > 0) {
                    tbody.innerHTML = data.databases.map(d => `<tr><td>${d.db_name}</td><td>${d.db_user}</td><td>${d.db_host}</td><td><button class='btn btn-sm btn-danger'><i class='bi bi-trash'></i></button></td></tr>`).join('');
                } else {
                    tbody.innerHTML = '<tr><td colspan="4" class="text-center text-muted">No databases created.</td></tr>';
                }
            } catch(e) {}
        }

        async function loadEmails() {
            try {
                const data = await apiFetch('/api/v1/user/emails?username=' + currentUser);
                const tbody = document.getElementById('emailTable');
                if(data.emails && data.emails.length > 0) {
                    tbody.innerHTML = data.emails.map(d => `<tr><td>${d.address}</td><td>${d.quota} MB</td><td>0 MB</td><td><button class='btn btn-sm btn-danger'><i class='bi bi-trash'></i></button></td></tr>`).join('');
                } else {
                    tbody.innerHTML = '<tr><td colspan="4" class="text-center text-muted">No mailboxes created.</td></tr>';
                }
            } catch(e) {}
        }

        document.getElementById('formAddDomain').addEventListener('submit', async (e) => {
            e.preventDefault();
            const btn = e.target.querySelector('button');
            const originalText = btn.innerText;
            btn.innerText = "Provisioning...";
            
            const payload = {
                username: currentUser,
                domain: document.getElementById('domName').value,
                php_version: document.getElementById('domPhp').value,
                ssl_enabled: document.getElementById('domSsl').checked
            };

            const data = await apiFetch('/api/v1/user/domains', 'POST', payload);
            
            if(res.ok) {
                alert("Domain provisioning queued!");
                await loadDomains();
            } else {
                alert("Error: " + (data.error || "Unknown logic error"));
            }

            btn.innerText = originalText;
            var myModalEl = document.getElementById('addDomainModal');
            var modal = bootstrap.Modal.getInstance(myModalEl);
            modal.hide();
            e.target.reset();
        });


        document.getElementById('formAddDb').addEventListener('submit', async (e) => {
            e.preventDefault();
            const btn = e.target.querySelector('button');
            const originalText = btn.innerText;
            btn.innerText = "Creating...";
            
            const payload = {
                username: currentUser,
                db_name: document.getElementById('dbName').value,
                db_user: document.getElementById('dbUser').value,
                db_pass: document.getElementById('dbPass').value
            };

            const data = await apiFetch('/api/v1/user/databases', 'POST', payload);
            
            if(res.ok) {
                alert("Database creation queued!");
                await loadDatabases();
            } else {
                alert("Error: " + (data.error || "Unknown logic error"));
            }

            btn.innerText = originalText;
            var myModalEl = document.getElementById('addDbModal');
            var modal = bootstrap.Modal.getInstance(myModalEl);
            modal.hide();
            e.target.reset();
        });

        document.getElementById('formAddEmail').addEventListener('submit', async (e) => {
            e.preventDefault();
            const btn = e.target.querySelector('button');
            const originalText = btn.innerText;
            btn.innerText = "Creating...";
            
            const payload = {
                username: currentUser,
                address: document.getElementById('emailAddress').value,
                password: document.getElementById('emailPass').value,
                quota: parseInt(document.getElementById('emailQuota').value, 10)
            };

            const data = await apiFetch('/api/v1/user/emails', 'POST', payload);
            
            if(res.ok) {
                alert("Mailbox creation queued!");
                await loadEmails();
            } else {
                alert("Error: " + (data.error || "Unknown logic error"));
            }

            btn.innerText = originalText;
            var myModalEl = document.getElementById('addEmailModal');
            var modal = bootstrap.Modal.getInstance(myModalEl);
            modal.hide();
            e.target.reset();
        });


        document.getElementById("formAddDb").addEventListener("submit", async (e) => {
            e.preventDefault();
            const btn = e.target.querySelector("button");
            const originalText = btn.innerText;
            btn.innerText = "Creating...";
            
            const payload = {
                username: currentUser,
                db_name: document.getElementById("dbName").value,
                db_user: document.getElementById("dbUser").value,
                db_pass: document.getElementById("dbPass").value
            };

            const data = await apiFetch('/api/v1/user/databases', 'POST', payload);
            
            if(res.ok) {
                alert("Database creation queued!");
                await loadDatabases();
            } else {
                alert("Error: " + (data.error || "Unknown logic"));
            }

            btn.innerText = originalText;
            var myModalEl = document.getElementById("addDbModal");
            var modal = bootstrap.Modal.getInstance(myModalEl);
            modal.hide();
            e.target.reset();
        });

        document.getElementById("formAddEmail").addEventListener("submit", async (e) => {
            e.preventDefault();
            const btn = e.target.querySelector("button");
            const originalText = btn.innerText;
            btn.innerText = "Creating...";
            
            const payload = {
                username: currentUser,
                address: document.getElementById("emailAddress").value,
                password: document.getElementById("emailPass").value,
                quota: parseInt(document.getElementById("emailQuota").value, 10)
            };

            const data = await apiFetch('/api/v1/user/emails', 'POST', payload);
            
            if(res.ok) {
                alert("Mailbox creation queued!");
                await loadEmails();
            } else {
                alert("Error: " + (data.error || "Unknown logic"));
            }

            btn.innerText = originalText;
            var myModalEl = document.getElementById("addEmailModal");
            var modal = bootstrap.Modal.getInstance(myModalEl);
            modal.hide();
            e.target.reset();
        });

        // Auto load on tab click
        document.querySelectorAll('a[data-bs-toggle="pill"]').forEach(tab => {
            tab.addEventListener('shown.bs.tab', event => {
                if(event.target.getAttribute('href') === '#domains') loadDomains();
                if(event.target.getAttribute('href') === '#databases') loadDatabases();
                if(event.target.getAttribute('href') === '#email') loadEmails();
            });
        });

        // initial load if appropriate
        loadDomains();
        // --- END REAL API FETCH LOGIC ---

        if (localStorage.getItem('adminReturn') === 'true') {
            document.getElementById('impersonationBar').classList.remove('d-none');
        }
