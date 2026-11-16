        // Offline base map: Natural Earth (public domain) GeoJSON served from
        // this Haven, so the map has land, water, roads and towns with no
        // internet. Added by the Haven ADS-B setup; listed first so it is the
        // default for a browser that has not picked a map yet.
        var neZoom = function (resolution) {
                return Math.log2(156543.03392804097 / resolution);
        };
        var neLayer = function (file, style) {
                return new ol.layer.Vector({
                        source: new ol.source.Vector({
                                url: 'geojson/offline/' + file,
                                format: new ol.format.GeoJSON()
                        }),
                        style: style,
                        declutter: file === 'places.geojson' || file === 'airports.geojson'
                });
        };
        var neWater = new ol.layer.Vector({
                source: new ol.source.Vector({
                        features: [new ol.Feature(ol.geom.Polygon.fromExtent(ol.proj.get('EPSG:3857').getExtent()))]
                }),
                style: new ol.style.Style({ fill: new ol.style.Fill({ color: '#aac8dc' }) })
        });
        var neLabel = function (text, size) {
                return new ol.style.Text({
                        text: text,
                        font: size + 'px sans-serif',
                        offsetY: -10,
                        fill: new ol.style.Fill({ color: '#333333' }),
                        stroke: new ol.style.Stroke({ color: '#ffffff', width: 3 })
                });
        };
        world.push(new ol.layer.Group({
                name: 'offline',
                title: 'Offline (Natural Earth)',
                type: 'base',
                combine: true,
                layers: [
                        neWater,
                        neLayer('land.geojson', new ol.style.Style({
                                fill: new ol.style.Fill({ color: '#efebe2' })
                        })),
                        neLayer('lakes.geojson', new ol.style.Style({
                                fill: new ol.style.Fill({ color: '#aac8dc' })
                        })),
                        neLayer('coast.geojson', new ol.style.Style({
                                stroke: new ol.style.Stroke({ color: '#7a9db3', width: 1 })
                        })),
                        neLayer('states.geojson', new ol.style.Style({
                                stroke: new ol.style.Stroke({ color: '#9b8fa0', width: 1, lineDash: [4, 4] })
                        })),
                        neLayer('roads.geojson', function (feature, resolution) {
                                var major = feature.get('type') === 'Major Highway';
                                if (!major && neZoom(resolution) < 8)
                                        return null;
                                return new ol.style.Style({
                                        stroke: new ol.style.Stroke({
                                                color: major ? '#d4894a' : '#dcb894',
                                                width: major ? 1.6 : 1
                                        })
                                });
                        }),
                        neLayer('places.geojson', function (feature, resolution) {
                                var rank = feature.get('scalerank');
                                var z = neZoom(resolution);
                                if (rank > (z < 7 ? 4 : z < 9 ? 6 : 8))
                                        return null;
                                return new ol.style.Style({
                                        image: new ol.style.Circle({
                                                radius: 2.5,
                                                fill: new ol.style.Fill({ color: '#555555' })
                                        }),
                                        text: neLabel(feature.get('name'), rank <= 3 ? 13 : 11)
                                });
                        }),
                        neLayer('airports.geojson', function (feature, resolution) {
                                if (neZoom(resolution) < 7)
                                        return null;
                                return new ol.style.Style({
                                        image: new ol.style.RegularShape({
                                                points: 4, radius: 4, angle: Math.PI / 4,
                                                fill: new ol.style.Fill({ color: '#6a5acd' })
                                        }),
                                        text: neLabel(feature.get('iata_code') || feature.get('abbrev') || '', 10)
                                });
                        })
                ]
        }));

