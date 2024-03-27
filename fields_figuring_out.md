# Get position (hdop, nsat, lon, lat, alt)
    position = {}
    nsat = None
    try:
        if modem == "ec2x":
            position = __salt__["ec2x.gnss_location"]()
            nsat = position.get('nsat', None)
        elif modem == "le910cx":
            position = __salt__["modem.connection"]("gnss_location", decimal_degrees=True)
            nsat = position.get('nsat_gps', None)
        log.info('Position: {}'.format(position))
    except Exception:
        log.warning('Exception when getting position, might be due to nofix')

    latitude = position.get('lat', None)
    longitude = position.get('lon', None)
    altitude = position.get('alt', None)
    hdop = position.get('hdop', None)

```bash
james@dimo.zone@John’s F\-150 XLT:2022 ford f\-150 $ modem.connection gnss_location decimal_degrees=True
_type: gnss_location
hdop: 0.8
time_utc: '02:04:27'
fix: 3D
lon: -113.507695
_stamp: '2024-02-28T02:04:27.471384'
nsat_gps: 7
date_utc: '2024-02-28'
cog: 12.4
lat: 53.485885
sog_kn: 0
alt: 665.2
sog_km: 0
nsat_glonass: 3
```

# get modem (to determin how to get above)

modem = __salt__["config.get"]("modem")

        if not modem:
            ec2x_success = False
            for x in range(1, 4):
                try:
                    _ = __salt__["ec2x.imei"]()
                    ec2x_success = True
                    break
                except Exception:
                    log.info("Ec2x imei failed {}/{}, expecting modem to be telit".format(x, 3))

            if ec2x_success:
                modem = "ec2x"
            else:
                modem = "le910cx"

```bash
james@dimo.zone@Volvo FH 24016:2023 volvo fh $ config.get modem
le910cx
```

# get signal lte info
lte ip addr

```bash
james@dimo.zone@Pz5V:2016 mitsubishi outlander $ network.ip_addrs wwan0
- 100.66.76.205
```

cell signal stuff
```shell
james@dimo.zone@Pz5V:2016 mitsubishi outlander $ qmi.signal_strength
rsrq:
  rating:
    text: fair
    value: 2
  unit: dB
  network: lte
  value: -15
  desc: Reference Signal Received Quality (4G LTE)
rsrp:
  rating:
    text: poor
    value: 1
  unit: dBm
  network: lte
  value: -112
  desc: Reference Signal Received Power (4G LTE)
ecio:
  rating:
    text: good
    value: 3
  unit: dBm
  network: lte
  value: -2.5
  desc: 'Energy to Interference Ratio (3G, CDMA/UMTS/EV-DO)'
sinr:
  rating:
    text: good
    value: 3
  unit: dB
  network: null
  value: 9
  desc: Signal to Interference-plus-Noise Ratio (4G LTE)
current:
  unit: dBm
  value: -79
  network: lte
snr:
  unit: dB
  value: 5.4
  network: lte
io:
  unit: dBm
  value: -106
  network: null
rssi:
  rating:
    text: fair
    value: 2
  unit: dBm
  network: lte
  value: -79
  desc: 'Received Signal Strength Indicator (3G, CDMA/UMTS/EV-DO)'
```

```shell
james@dimo.zone@Pz5V:2016 mitsubishi outlander $ qmi.cell_info
lte_info_neighboring_gsm:
  ue_in_idle: 'no'
interfrequency_lte_info:
  ue_in_idle: 'no'
intrafrequency_lte_info:
  'cell_[1]':
    rssi: '-93.4 dBm'
    physical_cell_id: 205
    rsrp: '-129.8 dBm'
    rsrq: '-20.0 dB'
  plmn: 311048
  global_cell_id: 4226326
  ue_in_idle: 'no'
  tracking_area_code: 4099
  'cell_[0]':
    rssi: '-79.1 dBm'
    physical_cell_id: 190
    rsrp: '-115.9 dBm'
    rsrq: '-17.0 dB'
  eutra_absolute_rf_channel_number: '2075 (E-UTRA band 4: AWS-1)'
  serving_cell_id: 190
  'cell_[2]':
    rssi: '-93.3 dBm'
    physical_cell_id: 343
    rsrp: '-123.6 dBm'
    rsrq: '-20.0 dB'
lte_info_neighboring_wcdma:
  ue_in_idle: 'no'
```
returned as json:
```json
{
    "lte_info_neighboring_gsm": {
        "ue_in_idle": "no"
    },
    "interfrequency_lte_info": {
        "ue_in_idle": "no"
    },
    "intrafrequency_lte_info": {
        "cell_[1]": {
            "rssi": "-93.4 dBm",
            "physical_cell_id": 205,
            "rsrp": "-129.8 dBm",
            "rsrq": "-20.0 dB"
        },
        "plmn": 311048,
        "global_cell_id": 4226326,
        "ue_in_idle": "no",
        "tracking_area_code": 4099,
        "cell_[0]": {
            "rssi": "-79.1 dBm",
            "physical_cell_id": 190,
            "rsrp": "-115.9 dBm",
            "rsrq": "-17.0 dB"
        },
        "eutra_absolute_rf_channel_number": "2075 (E-UTRA band 4: AWS-1)",
        "serving_cell_id": 190,
        "cell_[2]": {
            "rssi": "-93.3 dBm",
            "physical_cell_id": 343,
            "rsrp": "-123.6 dBm",
            "rsrq": "-20.0 dB"
        }
    },
    "lte_info_neighboring_wcdma": {
        "ue_in_idle": "no"
    }
}
```

what we currently save in elastic:
```json
{
  "cell": {
    "value": {
      "ip": "100.64.230.203",
      "details": {
        "plmn": 311048,
        "cell_[0]": {
          "rsrp": "-111.0 dBm",
          "rsrq": "-12.1 dB",
          "rssi": "-76.5 dBm",
          "physical_cell_id": 89
        },
        "cell_[1]": {
          "rsrp": "-116.2 dBm",
          "rsrq": "-18.1 dB",
          "rssi": "-89.9 dBm",
          "physical_cell_id": 404
        },
        "global_cell_id": 134159904
      }
    }
  }
}
```

# wifi networks
```shell
james@dimo.zone@Pz5V:2016 mitsubishi outlander $ wifi.scan
- name: CenturyLink5448
  encryption: WPA v.1
  signal_level: -64
  bit_rates: 1 Mb/s; 2 Mb/s; 5.5 Mb/s; 11 Mb/s; 18 Mb/s
  address: '9C:1E:95:03:8F:73'
  quality: 66
  channel: 1
- name: CenturyLink5448-5G
  encryption: WEP
  signal_level: -70
  bit_rates: 6 Mb/s; 9 Mb/s; 12 Mb/s; 18 Mb/s; 24 Mb/s
  address: '9C:1E:95:03:8F:76'
  quality: 57
  channel: 44
- name: ''
  encryption: WEP
  signal_level: -71
  bit_rates: 6 Mb/s; 9 Mb/s; 12 Mb/s; 18 Mb/s; 24 Mb/s
  address: '92:1E:95:03:8F:76'
  quality: 56
  channel: 44
- name: Making WiFi Great Again
  encryption: WEP
  signal_level: -81
  bit_rates: 1 Mb/s; 2 Mb/s; 5.5 Mb/s; 11 Mb/s; 6 Mb/s
  address: 'DA:6D:C9:58:18:D2'
  quality: 41
  channel: 6
```
for reference, above is returned as json from command as:
```json
[{
    "name": "CenturyLink5448",
    "encryption": "WPA v.1",
    "signal_level": -66,
    "bit_rates": "1 Mb/s; 2 Mb/s; 5.5 Mb/s; 11 Mb/s; 18 Mb/s",
    "address": "9C:1E:95:03:8F:73",
    "quality": 63,
    "channel": 1
}, {
    "name": "Making WiFi Great Again",
    "encryption": "WEP",
    "signal_level": -77,
    "bit_rates": "1 Mb/s; 2 Mb/s; 5.5 Mb/s; 11 Mb/s; 6 Mb/s",
    "address": "DA:6D:C9:58:18:D2",
    "quality": 47,
    "channel": 6
}, {
    "name": "NETGEAR67",
    "encryption": "WEP",
    "signal_level": -81,
    "bit_rates": "1 Mb/s; 2 Mb/s; 5.5 Mb/s; 11 Mb/s; 18 Mb/s",
    "address": "34:98:B5:D5:5A:AC",
    "quality": 41,
    "channel": 10
}]
```