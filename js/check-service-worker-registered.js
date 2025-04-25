(() => {
  if ('serviceWorker' in navigator) {
    navigator.serviceWorker.getRegistration()
      .then((registration) => {
        if (registration) {
          console.log('Service worker registered');
        } else {
          console.log('Service worker not registered');
        }
      })
      .catch((error) => {
        console.error('Error getting service worker registration:', error);
      });
  } else {
    console.log('Service worker not supported');
  }
})();
