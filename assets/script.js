;(function(){
  function initYoutubeEmbeds(){
    document.querySelectorAll(".youtube-embed:not([youtube-embed])").forEach(elm => {
      elm.setAttribute('youtube-embed', '');
      let url = elm.getAttribute('href');

      let query = 'autoplay=1&rel=0';
      if(url.includes('?')){
        url += '&'+query;
      }else{
        url += '?'+query;
      }

      let loaded = false;
      elm.addEventListener('click', function() {
        if(loaded){
          return;
        }
        loaded = true;

        playBtn = undefined;
        elm.querySelectorAll('img, h1').forEach(img => {
          if(img.classList.contains('youtube-embed-play-btn')){
            playBtn = img;
            playBtn.style['animation'] = 'youtube-embed-loading 0.75s ease-in-out infinite alternate';
            return;
          }
          img.remove();
        });

        const iframe = document.createElement('iframe');
        iframe.src = url;
        iframe.allow = 'accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture';
        iframe.setAttribute('allowfullscreen', '');
        if(playBtn){
          iframe.addEventListener('load', function() {
            playBtn.remove();
          });
        }
        elm.appendChild(iframe);
      });
    });
  }
  initYoutubeEmbeds();
  setInterval(initYoutubeEmbeds, 1000);

  function fixVideoRatios(){
    document.querySelectorAll('.youtube-embed[ratio]').forEach(elm => {
      let ratio = elm.getAttribute('ratio').split(':');
      ratio[0] = Number(ratio[0]);
      ratio[1] = Number(ratio[1]);

      elm.style["height"] = (elm.clientWidth * ratio[1] / ratio[0]) + 'px';
    });
  }
  fixVideoRatios();
  window.addEventListener('resize', fixVideoRatios);
})();
