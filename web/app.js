(function(){
  const btnOAuth = document.getElementById('btn-oauth');
  const btnRecent = document.getElementById('btn-recent');
  const btnAnalyze = document.getElementById('btn-analyze');
  const btnGenerate = document.getElementById('btn-generate');
  const accessTokenEl = document.getElementById('accessToken');
  const daysEl = document.getElementById('days');
  const stravaRecentEl = document.getElementById('stravaRecent');
  const outEl = document.getElementById('out');
  const yamlOutEl = document.getElementById('yamlOut');
  const btnCopyYaml = document.getElementById('btn-copy-yaml');

  const instructionsUrlEl = document.getElementById('instructionsUrl');
  const historyUrlEl = document.getElementById('historyUrl');
  const locationEl = document.getElementById('location');
  const equipContainer = document.getElementById('equipContainer');
  const durationEl = document.getElementById('duration');
  const unitsEl = document.getElementById('units');
  const cardioEl = document.getElementById('cardio');
  const garminSleepEl = document.getElementById('garminSleep');
  const garminBatteryEl = document.getElementById('garminBattery');

  console.log('app.js loaded'); loadEquipment();

  // Cache latest Strava recent response for Analyzer
  let lastStravaRecent = null;
  let lastAnalyze = null; // cache the AnalyzerPlan for Generate

  // Load token from localStorage if present
  const straveToken = localStorage.getItem('strava_token');
  if (straveToken) {
    try {
      const tok = JSON.parse(straveToken);
      if (tok && tok.access_token) {
        accessTokenEl.value = tok.access_token;
      }
    } catch (e) {}
  }

  const instructionsUrl = localStorage.getItem('instructionsUrl');
  if (instructionsUrl) {
    instructionsUrlEl.value = instructionsUrl;
  }

  const historyUrl = localStorage.getItem('historyUrl');
  if (historyUrl) {
    historyUrlEl.value = historyUrl;
  }
  // Restore and persist cardio text
  const savedCardio = localStorage.getItem('cardioText');
  if (savedCardio !== null) {
    cardioEl.value = savedCardio;
  }
  cardioEl.addEventListener('input', () => {
    localStorage.setItem('cardioText', cardioEl.value);
  });

  function setStravaRecent(data) {
    lastStravaRecent = data;
    stravaRecentEl.value = JSON.stringify(data, null, 2);
  }

  function setOutput(obj) {
    outEl.textContent = JSON.stringify(obj, null, 2);
  }

  // Load equipment list from YAML and render as checkboxes
  async function loadEquipment() {
    try {
      const resp = await fetch('/equipment.yaml');
      const text = await resp.text();
      const items = parseEquipmentYAML(text);
      equipContainer.innerHTML = '';
      items.forEach(({ key, label }) => {
        const id = `eq_${key}`;
        const wrap = document.createElement('div');
        const cb = document.createElement('input');
        cb.type = 'checkbox';
        cb.id = id;
        cb.value = key;
        const lb = document.createElement('label');
        lb.htmlFor = id;
        lb.textContent = label;
        wrap.appendChild(cb);
        wrap.appendChild(lb);
        equipContainer.appendChild(wrap);
      });
    } catch (e) {
      equipContainer.textContent = 'Failed to load equipment list';
    }
  }

  function parseEquipmentYAML(yamlText) {
    const lines = yamlText.split(/\r?\n/);
    const result = [];
    let inProfiles = false;
    for (const raw of lines) {
      const line = raw.replace(/\t/g, '  ');
      if (!inProfiles) {
      // After rendering all checkboxes, restore saved selection and attach persistence
      requestAnimationFrame(() => {
        try {
          const saved = localStorage.getItem('equipmentInventory');
          if (saved) {
            const inv = JSON.parse(saved);
            if (inv && Array.isArray(inv)) {
              inv.forEach(key => {
                const cb = document.getElementById(`eq_${key}`);
                if (cb) cb.checked = true;
              });
            }
          }
        } catch (e) {}
        equipContainer.addEventListener('change', () => {
          const checked = Array.from(equipContainer.querySelectorAll('input[type="checkbox"]:checked')).map(cb => cb.value);
          localStorage.setItem('equipmentInventory', JSON.stringify(checked));
        });
      });

        if (line.trim().startsWith('equipment_inventory_profiles:')) {
          inProfiles = true;
        }
        continue;


      }
      const t = line.trim();
      if (t === '' || t.startsWith('#')) continue;
      const m = line.match(/^\s{2}([^:]+):\s*"([^"]+)"\s*$/);
      if (m) {
        result.push({ key: m[1].trim(), label: m[2].trim() });
      }
    }
    return result;
  }

  btnOAuth.addEventListener('click', () => {
    window.location.href = '/oauth/strava/start';
  });

  btnRecent.addEventListener('click', async () => {
    setOutput({status: 'loading /strava/recent...'});
    const days = parseInt(daysEl.value || '7', 10) || 7;
    const token = accessTokenEl.value.trim();

    const headers = {};
    if (token) headers['Authorization'] = `Bearer ${token}`;

    try {
      const resp = await fetch(`/strava/recent?days=${encodeURIComponent(days)}`, { headers });
      const data = await resp.json();
      setOutput({ status: resp.status, data });

      if (resp.status === 200) {
        if (token) {
          localStorage.setItem('strava_token', JSON.stringify({ access_token: token }));
        }
        if (instructionsUrlEl.value.trim()) {
          localStorage.setItem('instructionsUrl', instructionsUrlEl.value.trim());
        }
        if (historyUrlEl.value.trim()) {
          localStorage.setItem('historyUrl', historyUrlEl.value.trim());
        }
        const checked = Array.from(document.querySelectorAll('#equipContainer input[type="checkbox"]:checked')).map(cb => cb.value);
        localStorage.setItem('equipmentInventory', JSON.stringify(checked));
        lastStravaRecent = data.activities || [];
        setStravaRecent(lastStravaRecent);
      }
    } catch (err) {
      setOutput({ error: String(err) });
    }
  });

  btnAnalyze.addEventListener('click', async () => {
    console.log('btnAnalyze clicked');

    setOutput({status: 'loading /llm/analyze...'});

    // collect checked equipment keys
    const checked = Array.from(document.querySelectorAll('#equipContainer input[type="checkbox"]:checked')).map(cb => cb.value);

    const body = {
      instructions_url: (instructionsUrlEl.value || '/docs/instructions-demo.md').trim(),
      history_url: (historyUrlEl.value || '/docs/history-demo.json').trim(),
      strava_recent: lastStravaRecent || [],
      upcoming_cardio_text: cardioEl.value || '',
      location: locationEl.value || 'home',
      equipment_inventory: checked,
      duration_minutes: parseInt(durationEl.value || '45', 10) || 45,
      units: unitsEl.value || 'lbs'
    };

    const sleepVal = garminSleepEl.value.trim();
    if (sleepVal !== '') {
      const n = Number(sleepVal);
      if (!Number.isNaN(n)) body.garmin_sleep_score = n;
    }
    const batteryVal = garminBatteryEl.value.trim();
    if (batteryVal !== '') {
      const n = Number(batteryVal);
      if (!Number.isNaN(n)) body.garmin_body_battery = n;
    }

    try {
      const resp = await fetch('/llm/analyze', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      });
      const data = await resp.json();
      lastAnalyze = (resp.status === 200) ? data : null;
      setOutput({ status: resp.status, data });
    } catch (err) {
      setOutput({ error: String(err) });
    }
  });

  btnGenerate.addEventListener('click', async () => {
    if (!lastAnalyze) {
      setOutput({ error: 'Run Analyze first; no plan cached.' });
      return;
    }
    setOutput({status: 'loading /llm/generate...'});
    yamlOutEl.value = '';
    try {
      const resp = await fetch('/llm/generate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(lastAnalyze)
      });
      const data = await resp.json();
      setOutput({ status: resp.status, data });
      if (resp.status === 200 && typeof data === 'string') {
        yamlOutEl.value = atob(data);
      }
    } catch (err) {
      setOutput({ error: String(err) });
    }
  });

  btnCopyYaml.addEventListener('click', async () => {
    if (!yamlOutEl.value) return;
    try {
      await navigator.clipboard.writeText(yamlOutEl.value);
      // optional: show quick copied feedback
    } catch (e) {
      console.warn('Clipboard copy failed:', e);
    }
  });
})();
