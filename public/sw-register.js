// sw-register.js
// Registers the Service Worker sw-zabbix.js without blocking the head

window.addEventListener('load', () => {
  if ('serviceWorker' in navigator) {
    navigator.serviceWorker.register('/sw.js', { scope: '/' })
      .then(reg => {
        console.log('Service Worker (/sw.js) registered:', reg);
      })
      .catch(err => {
        console.error('Service Worker (/sw.js) registration failed:', err);
      });
  }
});
