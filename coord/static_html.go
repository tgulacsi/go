/*
Copyright 2015 Tamás Gulácsi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package coord

const gmapsHTML = `<!DOCTYPE html>
<html>
  <head>
    <title>{{.Title}}</title>
    <meta name="viewport" content="initial-scale=1.0, user-scalable=no">
    <meta charset="utf-8">
    <style>
      html, body, #map-canvas {
        height: 100%;
        margin: 0px;
        padding: 0px
      }
    </style>
 <style>
      html, body, #map-canvas {
        height: 100%;
        margin: 0px;
        padding: 0px
      }
      .controls {
        margin-top: 16px;
        border: 1px solid transparent;
        border-radius: 2px 0 0 2px;
        box-sizing: border-box;
        -moz-box-sizing: border-box;
        height: 32px;
        outline: none;
        box-shadow: 0 2px 6px rgba(0, 0, 0, 0.3);
      }

      #pac-input {
        background-color: #fff;
        padding: 0 11px 0 13px;
        width: 400px;
        font-family: Roboto;
        font-size: 15px;
        font-weight: 300;
        text-overflow: ellipsis;
      }

      #pac-input:focus {
        border-color: #4d90fe;
        margin-left: -1px;
        padding-left: 14px;  /* Regular padding-left + 1. */
        width: 401px;
      }

      .pac-container {
        font-family: Roboto;
      }

      #type-selector {
        color: #fff;
        background-color: #4d90fe;
        padding: 5px 11px 0px 11px;
      }

      #type-selector label {
        font-family: Roboto;
        font-size: 13px;
        font-weight: 300;
      }
}
    </style>
    <script src="https://maps.googleapis.com/maps/api/js?v=3.exp&libraries=places"></script>
    <script>
var map;
var infowindow;
var zoomed = false;
var cbPath = "{{.CallbackPath}}";

function encode_utf8(s) {
  return unescape(encodeURIComponent(s));
}

function decode_utf8(s) {
  return decodeURIComponent(escape(s));
}

function initialize() {
  var QueryParameters = (function() {
    var result = {};
    if (window.location.search) {
      var params = window.location.search.slice(1).split("&");
      for (var i = 0; i < params.length; i++) {
        var tmp = params[i].split("=");
        result[tmp[0]] = decodeURIComponent(tmp[1]);
      }
    }
    return result;
  }());

  var mapOptions = {
    zoom: 8,
    center: new google.maps.LatLng({{.MapCenterLat}}, {{.MapCenterLng}}),
    streetViewControl: false
  };
  map = new google.maps.Map(document.getElementById('map-canvas'),
      mapOptions);

  // Create the search box and link it to the UI element.
  var input = /** @type {HTMLInputElement} */(
      document.getElementById('pac-input'));
  map.controls[google.maps.ControlPosition.TOP_LEFT].push(input);

  var request = {
    location: new google.maps.LatLng({{.LocLat}}, {{.LocLng}}),
    radius: 257 * 1000,
    query: QueryParameters.address
  }
  var service = new google.maps.places.PlacesService(map);
  service.textSearch(request, callback);
  input.value = QueryParameters.address == "" ? "{{.DefaultAddress}}" : QueryParameters.address;

  infowindow = new google.maps.InfoWindow();
  var i = 0;
  for (var key in QueryParameters) {
    if (key == "lat" || key == "lng") {
      continue;
    }
    cbPath += (i == 0 ? "?" : "&") + key + "=" + encodeURIComponent(QueryParameters[key]);
    i++;
  }
  console.log( "cbPath=" + cbPath );

  var searchBox = new google.maps.places.SearchBox(
    /** @type {HTMLInputElement} */(input));

  // Listen for the event fired when the user selects an item from the
  // pick list. Retrieve the matching places for that item.
  google.maps.event.addListener(searchBox, 'places_changed', function() {
    var places = searchBox.getPlaces();

    if (places.length == 0) {
      return;
    }
	createMarker(places[0]);
  });

  // Bias the SearchBox results towards places that are within the bounds of the
  // current map's viewport.
  google.maps.event.addListener(map, 'bounds_changed', function() {
    var bounds = map.getBounds();
    searchBox.setBounds(bounds);
  });
}

function callback(results, status) {
  if (status != google.maps.places.PlacesServiceStatus.OK || results.length == 0) {
    return;
  }
  createMarker(results[0]);
}

function createMarker(place) {
  var placeLoc = place.geometry.location;
  map.setCenter(placeLoc);
  if (!zoomed) {
    zoomed = true;
    map.setZoom(14);
  }
  var marker = new google.maps.Marker({
    map: map,
    position: placeLoc,
    draggable: true
  });

  pos = marker.getPosition();
  infowindow.setContent(
      '<a href="' + cbPath + "&lat=" + pos.lat() + "&lng=" + pos.lng() + '">' +
      place.name + ' ' + pos + '</a>');
  infowindow.open(map, marker);

  google.maps.event.addListener(marker, 'dblclick', function() {
    alert("send " + marker.getPosition() + " to " + encodeURIComponent(cbPath));
  })
}

google.maps.event.addDomListener(window, 'load', initialize);

    </script>
  </head>
  <body>
	<input id="pac-input" class="controls" type="text" placeholder="Search Box">
    <div id="map-canvas"></div>
  </body>
</html>
`
