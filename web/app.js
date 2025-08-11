(function(){
  const btnOAuth = document.getElementById('btn-oauth');
  const btnRecent = document.getElementById('btn-recent');
  const accessTokenEl = document.getElementById('accessToken');
  const daysEl = document.getElementById('days');
  const outEl = document.getElementById('out');

  // Load token from localStorage if present
  const saved = localStorage.getItem('strava_token');
  if (saved) {
    try {
      const tok = JSON.parse(saved);
      if (tok && tok.access_token) {
        accessTokenEl.value = tok.access_token;
      }
    } catch (e) {}
  }

  function setOutput(obj) {
    outEl.textContent = JSON.stringify(obj, null, 2);
  }

  btnOAuth.addEventListener('click', () => {
    // Kick off the OAuth flow via server route
    window.location.href = '/oauth/strava/start';
  });

  btnRecent.addEventListener('click', async () => {
    setOutput({status: 'loading...'});
    const days = parseInt(daysEl.value || '7', 10) || 7;
    const token = accessTokenEl.value.trim();

    const headers = {};
    if (token) headers['Authorization'] = `Bearer ${token}`;

    try {
      const resp = await fetch(`/strava/recent?days=${encodeURIComponent(days)}`, { headers });
      const data = await resp.json();
      setOutput({ status: resp.status, data });

      if (resp.status === 200) {
        // If the app later includes refreshed tokens in responses, persist them here.
        // For now, just persist the current access token for convenience.
        if (token) {
          localStorage.setItem('strava_token', JSON.stringify({ access_token: token }));
        }
      } else if (resp.status === 401 && data && data.oauth_url) {
        // suggest user to authenticate
        // Optionally, redirect automatically:
        // window.location.href = data.oauth_url;
      }
    } catch (err) {
      setOutput({ error: String(err) });
    }
  });
})();

