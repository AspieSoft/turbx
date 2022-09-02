;(function(){
  async function initClientYoutubeEmbed(elm){
    let url = elm.getAttribute('src');
    if(!url || url === ''){
      return false;
    }

    const videoData = {};
    if(url.startsWith('c/')){
      //todo: get embed url for custom channel url
      // url = url.replace('c/', '');
      elm.remove();
      return false;
    }else if(url.startsWith('UC') || url.startsWith('UU')){
      let vidUrl = url.replace(/^U[CU]/, '');

      //todo: Check if video can be embedded

      vidUrl = 'UU' + vidUrl;

      const res = await fetch('https://www.youtube.com/oembed?url=https://www.youtube.com/playlist?list='+vidUrl+'&format=json');
      if(!res || !res.ok){
        elm.remove();
        return false;
      }
      let body;
      try {
        body = await res.json();
      } catch(e) {
        elm.remove();
        return false;
      }

      videoData['url'] = 'https://www.youtube.com/embed/?list=' + url;
      
      if(body['thumbnail_url']){
        videoData['img'] = body['thumbnail_url'].toString();
      }
      if(body['title']){
        videoData['title'] = body['title'].toString();
      }
      if(body['width'] && body['height']){
        videoData['ratio'] = body['width'].toString() + ':' + body['height'].toString();
      }
    }else if(url.startsWith('PU') || url.startsWith('PL')){
      const res = await fetch('https://www.youtube.com/oembed?url=https://www.youtube.com/playlist?list='+url+'&format=json');
      if(!res || !res.ok){
        elm.remove();
        return false;
      }
      let body;
      try {
        body = await res.json();
      } catch(e) {
        elm.remove();
        return false;
      }

      videoData['url'] = 'https://www.youtube.com/embed/?list=' + url;
      
      if(body['thumbnail_url']){
        videoData['img'] = body['thumbnail_url'].toString();
      }
      if(body['title']){
        videoData['title'] = body['title'].toString();
      }
      if(body['width'] && body['height']){
        videoData['ratio'] = body['width'].toString() + ':' + body['height'].toString();
      }
    }else{
      const res = await fetch('https://www.youtube.com/oembed?url=https://www.youtube.com/watch?v='+url+'&format=json');
      if(!res || !res.ok){
        elm.remove();
        return false;
      }
      let body;
      try {
        body = await res.json();
      } catch(e) {
        elm.remove();
        return false;
      }

      videoData['url'] = 'https://www.youtube.com/embed/' + url;

      if(body['thumbnail_url']){
        videoData['img'] = body['thumbnail_url'].toString();
      }
      if(body['title']){
        videoData['title'] = body['title'].toString();
      }
      if(body['width'] && body['height']){
        videoData['ratio'] = body['width'].toString() + ':' + body['height'].toString();
      }
    }

    if(!videoData['url']){
      elm.remove();
      return false;
    }

    if(videoData['ratio']){
      elm.setAttribute('ratio', videoData['ratio']);

      let ratio = videoData['ratio'].split(':');
      ratio[0] = Number(ratio[0]);
      ratio[1] = Number(ratio[1]);

      elm.style["height"] = (elm.clientWidth * ratio[1] / ratio[0]) + 'px';
    }
    if(videoData['img']){
      const img = document.createElement('img');
      img.src = videoData['img'];
      img.alt = 'YouTube Embed';
      elm.appendChild(img);
    }
    if(videoData['title']){
      const title = document.createElement('h1');
      title.textContent = videoData['title'];
      elm.appendChild(title);
    }
    const playBtn = elm.querySelector('.youtube-embed-play-btn');
    if(playBtn){
      elm.appendChild(playBtn);
    }
    elm.setAttribute('href', videoData['url']);

    return true;
  }

  function initYoutubeEmbeds(){
    document.querySelectorAll(".youtube-embed:not([youtube-embed])").forEach(async (elm) => {
      elm.setAttribute('youtube-embed', '');

      if(elm.classList.contains('youtube-embed-client')){
        let success = await initClientYoutubeEmbed(elm);
        if(!success){
          return;
        }
      }

      let url = elm.getAttribute('href');
      if(!url || url === ''){
        return false;
      }

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
