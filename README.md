<div align="center">
  <img src="./logo.png" width="450" />
  
  # Watchtower
  
  A process for automating Docker container base image updates.
  <br/><br/>
  
  [![Circle CI](https://circleci.com/gh/containrrr/watchtower.svg?style=shield)](https://circleci.com/gh/containrrr/watchtower)
  [![codecov](https://codecov.io/gh/containrrr/watchtower/branch/main/graph/badge.svg)](https://codecov.io/gh/containrrr/watchtower)
  [![GoDoc](https://godoc.org/github.com/containrrr/watchtower?status.svg)](https://godoc.org/github.com/containrrr/watchtower)
  [![Go Report Card](https://goreportcard.com/badge/github.com/containrrr/watchtower)](https://goreportcard.com/report/github.com/containrrr/watchtower)
  [![latest version](https://img.shields.io/github/tag/containrrr/watchtower.svg)](https://github.com/containrrr/watchtower/releases)
  [![Apache-2.0 License](https://img.shields.io/github/license/containrrr/watchtower.svg)](https://www.apache.org/licenses/LICENSE-2.0)
  [![Codacy Badge](https://app.codacy.com/project/badge/Grade/1c48cfb7646d4009aa8c6f71287670b8)](https://www.codacy.com/gh/containrrr/watchtower/dashboard?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=containrrr/watchtower&amp;utm_campaign=Badge_Grade)
  [![All Contributors](https://img.shields.io/github/all-contributors/containrrr/watchtower)](#contributors)
  [![Pulls from DockerHub](https://img.shields.io/docker/pulls/containrrr/watchtower.svg)](https://hub.docker.com/r/containrrr/watchtower)

</div>

## Quick Start

With watchtower you can update the running version of your containerized app simply by pushing a new image to the Docker Hub or your own image registry. 

Watchtower will pull down your new image, gracefully shut down your existing container and restart it with the same options that were used when it was deployed initially. Run the watchtower container with the following command:

```
$ docker run --detach \
    --name watchtower \
    --volume /var/run/docker.sock:/var/run/docker.sock \
    containrrr/watchtower
```

## Documentation
The full documentation is available at https://containrrr.dev/watchtower.

## Contributors

Thanks goes to these wonderful people ([emoji key](https://allcontributors.org/docs/en/emoji-key)):

<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->
<table>
  <tr>
    <td align="center"><a href="http://codelica.com"><img src="https://avatars3.githubusercontent.com/u/386101?v=4?s=100" width="100px;" alt=""/><br /><sub><b>James</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=Codelica" title="Tests">⚠️</a> <a href="#ideas-Codelica" title="Ideas, Planning, & Feedback">🤔</a></td>
    <td align="center"><a href="https://kopfkrieg.org"><img src="https://avatars2.githubusercontent.com/u/5047813?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Florian</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/pulls?q=is%3Apr+reviewed-by%3AKopfKrieg" title="Reviewed Pull Requests">👀</a> <a href="https://github.com/containrrr/watchtower/commits?author=KopfKrieg" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/bdehamer"><img src="https://avatars1.githubusercontent.com/u/398027?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Brian DeHamer</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=bdehamer" title="Code">💻</a> <a href="#maintenance-bdehamer" title="Maintenance">🚧</a></td>
    <td align="center"><a href="https://github.com/rosscado"><img src="https://avatars1.githubusercontent.com/u/16578183?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Ross Cadogan</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=rosscado" title="Code">💻</a></td>
    <td align="center"><a href="https://github.com/stffabi"><img src="https://avatars0.githubusercontent.com/u/9464631?v=4?s=100" width="100px;" alt=""/><br /><sub><b>stffabi</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=stffabi" title="Code">💻</a> <a href="#maintenance-stffabi" title="Maintenance">🚧</a></td>
    <td align="center"><a href="https://github.com/ATCUSA"><img src="https://avatars3.githubusercontent.com/u/3581228?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Austin</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=ATCUSA" title="Documentation">📖</a></td>
    <td align="center"><a href="https://labs.ctl.io"><img src="https://avatars2.githubusercontent.com/u/6181487?v=4?s=100" width="100px;" alt=""/><br /><sub><b>David Gardner</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/pulls?q=is%3Apr+reviewed-by%3Adavidgardner11" title="Reviewed Pull Requests">👀</a> <a href="https://github.com/containrrr/watchtower/commits?author=davidgardner11" title="Documentation">📖</a></td>
  </tr>
  <tr>
    <td align="center"><a href="https://github.com/dolanor"><img src="https://avatars3.githubusercontent.com/u/928722?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Tanguy ⧓ Herrmann</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=dolanor" title="Code">💻</a></td>
    <td align="center"><a href="https://github.com/rdamazio"><img src="https://avatars3.githubusercontent.com/u/997641?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Rodrigo Damazio Bovendorp</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=rdamazio" title="Code">💻</a> <a href="https://github.com/containrrr/watchtower/commits?author=rdamazio" title="Documentation">📖</a></td>
    <td align="center"><a href="https://www.taisun.io/"><img src="https://avatars3.githubusercontent.com/u/1852688?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Ryan Kuba</b></sub></a><br /><a href="#infra-thelamer" title="Infrastructure (Hosting, Build-Tools, etc)">🚇</a></td>
    <td align="center"><a href="https://github.com/cnrmck"><img src="https://avatars2.githubusercontent.com/u/22061955?v=4?s=100" width="100px;" alt=""/><br /><sub><b>cnrmck</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=cnrmck" title="Documentation">📖</a></td>
    <td align="center"><a href="http://harrywalter.co.uk"><img src="https://avatars3.githubusercontent.com/u/338588?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Harry Walter</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=haswalt" title="Code">💻</a></td>
    <td align="center"><a href="http://projectsperanza.com"><img src="https://avatars3.githubusercontent.com/u/74515?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Robotex</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=Robotex" title="Documentation">📖</a></td>
    <td align="center"><a href="http://geraldpape.io"><img src="https://avatars0.githubusercontent.com/u/1494211?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Gerald Pape</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=ubergesundheit" title="Documentation">📖</a></td>
  </tr>
  <tr>
    <td align="center"><a href="https://github.com/fomk"><img src="https://avatars0.githubusercontent.com/u/17636183?v=4?s=100" width="100px;" alt=""/><br /><sub><b>fomk</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=fomk" title="Code">💻</a></td>
    <td align="center"><a href="https://github.com/svengo"><img src="https://avatars3.githubusercontent.com/u/2502366?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Sven Gottwald</b></sub></a><br /><a href="#infra-svengo" title="Infrastructure (Hosting, Build-Tools, etc)">🚇</a></td>
    <td align="center"><a href="https://liberapay.com/techknowlogick/"><img src="https://avatars1.githubusercontent.com/u/164197?v=4?s=100" width="100px;" alt=""/><br /><sub><b>techknowlogick</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=techknowlogick" title="Code">💻</a></td>
    <td align="center"><a href="http://log.c5t.org/about/"><img src="https://avatars1.githubusercontent.com/u/1449568?v=4?s=100" width="100px;" alt=""/><br /><sub><b>waja</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=waja" title="Documentation">📖</a></td>
    <td align="center"><a href="http://scottalbertson.com"><img src="https://avatars2.githubusercontent.com/u/154463?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Scott Albertson</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=salbertson" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/huddlesj"><img src="https://avatars1.githubusercontent.com/u/11966535?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Jason Huddleston</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=huddlesj" title="Documentation">📖</a></td>
    <td align="center"><a href="https://npstr.space/"><img src="https://avatars3.githubusercontent.com/u/6048348?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Napster</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=napstr" title="Code">💻</a></td>
  </tr>
  <tr>
    <td align="center"><a href="https://github.com/darknode"><img src="https://avatars1.githubusercontent.com/u/809429?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Maxim</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=darknode" title="Code">💻</a> <a href="https://github.com/containrrr/watchtower/commits?author=darknode" title="Documentation">📖</a></td>
    <td align="center"><a href="https://schmitt.cat"><img src="https://avatars0.githubusercontent.com/u/17984549?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Max Schmitt</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=mxschmitt" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/cron410"><img src="https://avatars1.githubusercontent.com/u/3082899?v=4?s=100" width="100px;" alt=""/><br /><sub><b>cron410</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=cron410" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/Cardoso222"><img src="https://avatars3.githubusercontent.com/u/7026517?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Paulo Henrique</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=Cardoso222" title="Documentation">📖</a></td>
    <td align="center"><a href="https://coded.io"><img src="https://avatars0.githubusercontent.com/u/107097?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Kaleb Elwert</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=belak" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/wmbutler"><img src="https://avatars1.githubusercontent.com/u/1254810?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Bill Butler</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=wmbutler" title="Documentation">📖</a></td>
    <td align="center"><a href="https://www.mariotacke.io"><img src="https://avatars2.githubusercontent.com/u/4942019?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Mario Tacke</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=mariotacke" title="Code">💻</a></td>
  </tr>
  <tr>
    <td align="center"><a href="https://markwoodbridge.com"><img src="https://avatars2.githubusercontent.com/u/1101318?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Mark Woodbridge</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=mrw34" title="Code">💻</a></td>
    <td align="center"><a href="http://simme.dev"><img src="https://avatars0.githubusercontent.com/u/1596025?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Simon Aronsson</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=simskij" title="Code">💻</a> <a href="#maintenance-simskij" title="Maintenance">🚧</a> <a href="https://github.com/containrrr/watchtower/pulls?q=is%3Apr+reviewed-by%3Asimskij" title="Reviewed Pull Requests">👀</a> <a href="https://github.com/containrrr/watchtower/commits?author=simskij" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/Ansem93"><img src="https://avatars3.githubusercontent.com/u/6626218?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Ansem93</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=Ansem93" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/lukapeschke"><img src="https://avatars1.githubusercontent.com/u/17085536?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Luka Peschke</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=lukapeschke" title="Code">💻</a> <a href="https://github.com/containrrr/watchtower/commits?author=lukapeschke" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/zoispag"><img src="https://avatars0.githubusercontent.com/u/21138205?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Zois Pagoulatos</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=zoispag" title="Code">💻</a> <a href="https://github.com/containrrr/watchtower/pulls?q=is%3Apr+reviewed-by%3Azoispag" title="Reviewed Pull Requests">👀</a> <a href="#maintenance-zoispag" title="Maintenance">🚧</a></td>
    <td align="center"><a href="https://alexandre.menif.name"><img src="https://avatars0.githubusercontent.com/u/16152103?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Alexandre Menif</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=alexandremenif" title="Code">💻</a></td>
    <td align="center"><a href="https://github.com/chugunov"><img src="https://avatars1.githubusercontent.com/u/4140479?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Andrey</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=chugunov" title="Documentation">📖</a></td>
  </tr>
  <tr>
    <td align="center"><a href="https://noplanman.ch"><img src="https://avatars3.githubusercontent.com/u/9423417?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Armando Lüscher</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=noplanman" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/rjbudke"><img src="https://avatars2.githubusercontent.com/u/273485?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Ryan Budke</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=rjbudke" title="Documentation">📖</a></td>
    <td align="center"><a href="http://kaloyan.raev.name"><img src="https://avatars2.githubusercontent.com/u/468091?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Kaloyan Raev</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=kaloyan-raev" title="Code">💻</a> <a href="https://github.com/containrrr/watchtower/commits?author=kaloyan-raev" title="Tests">⚠️</a></td>
    <td align="center"><a href="https://github.com/sixth"><img src="https://avatars3.githubusercontent.com/u/11591445?v=4?s=100" width="100px;" alt=""/><br /><sub><b>sixth</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=sixth" title="Documentation">📖</a></td>
    <td align="center"><a href="https://foosel.net"><img src="https://avatars0.githubusercontent.com/u/83657?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Gina Häußge</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=foosel" title="Code">💻</a></td>
    <td align="center"><a href="https://github.com/8ear"><img src="https://avatars0.githubusercontent.com/u/10329648?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Max H.</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=8ear" title="Code">💻</a></td>
    <td align="center"><a href="https://pjknkda.github.io"><img src="https://avatars0.githubusercontent.com/u/4986524?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Jungkook Park</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=pjknkda" title="Documentation">📖</a></td>
  </tr>
  <tr>
    <td align="center"><a href="https://achfrag.net"><img src="https://avatars1.githubusercontent.com/u/5753622?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Jan Kristof Nidzwetzki</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=jnidzwetzki" title="Documentation">📖</a></td>
    <td align="center"><a href="https://www.lukaselsner.de"><img src="https://avatars0.githubusercontent.com/u/1413542?v=4?s=100" width="100px;" alt=""/><br /><sub><b>lukas</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=mindrunner" title="Code">💻</a></td>
    <td align="center"><a href="https://codingcoffee.dev"><img src="https://avatars3.githubusercontent.com/u/13611153?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Ameya Shenoy</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=codingCoffee" title="Code">💻</a></td>
    <td align="center"><a href="https://github.com/raymondelooff"><img src="https://avatars0.githubusercontent.com/u/9716806?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Raymon de Looff</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=raymondelooff" title="Code">💻</a></td>
    <td align="center"><a href="http://codemonkeylabs.com"><img src="https://avatars2.githubusercontent.com/u/704034?v=4?s=100" width="100px;" alt=""/><br /><sub><b>John Clayton</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=jsclayton" title="Code">💻</a></td>
    <td align="center"><a href="https://github.com/Germs2004"><img src="https://avatars2.githubusercontent.com/u/5519340?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Germs2004</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=Germs2004" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/lukwil"><img src="https://avatars1.githubusercontent.com/u/30203234?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Lukas Willburger</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=lukwil" title="Code">💻</a></td>
  </tr>
  <tr>
    <td align="center"><a href="https://github.com/auanasgheps"><img src="https://avatars2.githubusercontent.com/u/20586878?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Oliver Cervera</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=auanasgheps" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/victorcmoura"><img src="https://avatars1.githubusercontent.com/u/26290053?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Victor Moura</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=victorcmoura" title="Tests">⚠️</a> <a href="https://github.com/containrrr/watchtower/commits?author=victorcmoura" title="Code">💻</a> <a href="https://github.com/containrrr/watchtower/commits?author=victorcmoura" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/mbrandau"><img src="https://avatars3.githubusercontent.com/u/12972798?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Maximilian Brandau</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=mbrandau" title="Code">💻</a> <a href="https://github.com/containrrr/watchtower/commits?author=mbrandau" title="Tests">⚠️</a></td>
    <td align="center"><a href="https://github.com/aneisch"><img src="https://avatars1.githubusercontent.com/u/6991461?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Andrew</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=aneisch" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/sixcorners"><img src="https://avatars0.githubusercontent.com/u/585501?v=4?s=100" width="100px;" alt=""/><br /><sub><b>sixcorners</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=sixcorners" title="Documentation">📖</a></td>
    <td align="center"><a href="https://piksel.se"><img src="https://avatars2.githubusercontent.com/u/807383?v=4?s=100" width="100px;" alt=""/><br /><sub><b>nils måsén</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=piksel" title="Documentation">📖</a> <a href="https://github.com/containrrr/watchtower/commits?author=piksel" title="Code">💻</a></td>
    <td align="center"><a href="https://arnested.dk"><img src="https://avatars2.githubusercontent.com/u/190005?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Arne Jørgensen</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=arnested" title="Tests">⚠️</a> <a href="https://github.com/containrrr/watchtower/pulls?q=is%3Apr+reviewed-by%3Aarnested" title="Reviewed Pull Requests">👀</a></td>
  </tr>
  <tr>
    <td align="center"><a href="https://github.com/patski123"><img src="https://avatars1.githubusercontent.com/u/19295295?v=4?s=100" width="100px;" alt=""/><br /><sub><b>PatSki123</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=patski123" title="Documentation">📖</a></td>
    <td align="center"><a href="https://rubyroidlabs.com/"><img src="https://avatars2.githubusercontent.com/u/624999?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Valentine Zavadsky</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=Saicheg" title="Code">💻</a> <a href="https://github.com/containrrr/watchtower/commits?author=Saicheg" title="Documentation">📖</a> <a href="https://github.com/containrrr/watchtower/commits?author=Saicheg" title="Tests">⚠️</a></td>
    <td align="center"><a href="https://github.com/bopoh24"><img src="https://avatars2.githubusercontent.com/u/4086631?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Alexander Voronin</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=bopoh24" title="Code">💻</a> <a href="https://github.com/containrrr/watchtower/issues?q=author%3Abopoh24" title="Bug reports">🐛</a></td>
    <td align="center"><a href="http://www.teqneers.de"><img src="https://avatars0.githubusercontent.com/u/788989?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Oliver Mueller</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=ogmueller" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/tammert"><img src="https://avatars0.githubusercontent.com/u/8885250?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Sebastiaan Tammer</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=tammert" title="Code">💻</a></td>
    <td align="center"><a href="https://github.com/Miosame"><img src="https://avatars1.githubusercontent.com/u/8201077?v=4?s=100" width="100px;" alt=""/><br /><sub><b>miosame</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=miosame" title="Documentation">📖</a></td>
    <td align="center"><a href="https://mtz.gr"><img src="https://avatars3.githubusercontent.com/u/590246?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Andrew Metzger</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/issues?q=author%3Aandrewjmetzger" title="Bug reports">🐛</a> <a href="#example-andrewjmetzger" title="Examples">💡</a></td>
  </tr>
  <tr>
    <td align="center"><a href="https://github.com/pgrimaud"><img src="https://avatars1.githubusercontent.com/u/1866496?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Pierre Grimaud</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=pgrimaud" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/mattdoran"><img src="https://avatars0.githubusercontent.com/u/577779?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Matt Doran</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=mattdoran" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/MihailITPlace"><img src="https://avatars2.githubusercontent.com/u/28401551?v=4?s=100" width="100px;" alt=""/><br /><sub><b>MihailITPlace</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=MihailITPlace" title="Code">💻</a></td>
    <td align="center"><a href="https://github.com/bugficks"><img src="https://avatars1.githubusercontent.com/u/2992895?v=4?s=100" width="100px;" alt=""/><br /><sub><b>bugficks</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=bugficks" title="Code">💻</a> <a href="https://github.com/containrrr/watchtower/commits?author=bugficks" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/MichaelSp"><img src="https://avatars0.githubusercontent.com/u/448282?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Michael</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=MichaelSp" title="Code">💻</a></td>
    <td align="center"><a href="https://github.com/jokay"><img src="https://avatars0.githubusercontent.com/u/18613935?v=4?s=100" width="100px;" alt=""/><br /><sub><b>D. Domig</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=jokay" title="Documentation">📖</a></td>
    <td align="center"><a href="https://maxwells-daemon.io"><img src="https://avatars1.githubusercontent.com/u/260084?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Ben Osheroff</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=osheroff" title="Code">💻</a></td>
  </tr>
  <tr>
    <td align="center"><a href="https://github.com/dhet"><img src="https://avatars3.githubusercontent.com/u/2668621?v=4?s=100" width="100px;" alt=""/><br /><sub><b>David H.</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=dhet" title="Code">💻</a></td>
    <td align="center"><a href="http://www.gridgeo.com"><img src="https://avatars1.githubusercontent.com/u/671887?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Chander Ganesan</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=chander" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/yrien30"><img src="https://avatars1.githubusercontent.com/u/26816162?v=4?s=100" width="100px;" alt=""/><br /><sub><b>yrien30</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=yrien30" title="Code">💻</a></td>
    <td align="center"><a href="https://github.com/ksurl"><img src="https://avatars1.githubusercontent.com/u/1371562?v=4?s=100" width="100px;" alt=""/><br /><sub><b>ksurl</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=ksurl" title="Documentation">📖</a> <a href="https://github.com/containrrr/watchtower/commits?author=ksurl" title="Code">💻</a> <a href="#infra-ksurl" title="Infrastructure (Hosting, Build-Tools, etc)">🚇</a></td>
    <td align="center"><a href="https://github.com/rg9400"><img src="https://avatars2.githubusercontent.com/u/39887349?v=4?s=100" width="100px;" alt=""/><br /><sub><b>rg9400</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=rg9400" title="Code">💻</a></td>
    <td align="center"><a href="https://github.com/tkalus"><img src="https://avatars2.githubusercontent.com/u/287181?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Turtle Kalus</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=tkalus" title="Code">💻</a></td>
    <td align="center"><a href="https://github.com/SrihariThalla"><img src="https://avatars1.githubusercontent.com/u/7479937?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Srihari Thalla</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=SrihariThalla" title="Documentation">📖</a></td>
  </tr>
  <tr>
    <td align="center"><a href="https://nymous.io"><img src="https://avatars1.githubusercontent.com/u/4216559?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Thomas Gaudin</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=nymous" title="Documentation">📖</a></td>
    <td align="center"><a href="https://indigo.re/"><img src="https://avatars.githubusercontent.com/u/2804645?v=4?s=100" width="100px;" alt=""/><br /><sub><b>hydrargyrum</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=hydrargyrum" title="Documentation">📖</a></td>
    <td align="center"><a href="https://reinout.vanrees.org"><img src="https://avatars.githubusercontent.com/u/121433?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Reinout van Rees</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=reinout" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/DasSkelett"><img src="https://avatars.githubusercontent.com/u/28812678?v=4?s=100" width="100px;" alt=""/><br /><sub><b>DasSkelett</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=DasSkelett" title="Code">💻</a></td>
    <td align="center"><a href="https://github.com/zenjabba"><img src="https://avatars.githubusercontent.com/u/679864?v=4?s=100" width="100px;" alt=""/><br /><sub><b>zenjabba</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=zenjabba" title="Documentation">📖</a></td>
    <td align="center"><a href="https://quan.io"><img src="https://avatars.githubusercontent.com/u/3526705?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Dan Quan</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=djquan" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/modem7"><img src="https://avatars.githubusercontent.com/u/4349962?v=4?s=100" width="100px;" alt=""/><br /><sub><b>modem7</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=modem7" title="Documentation">📖</a></td>
  </tr>
  <tr>
    <td align="center"><a href="https://github.com/hypnoglow"><img src="https://avatars.githubusercontent.com/u/4853075?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Igor Zibarev</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=hypnoglow" title="Code">💻</a></td>
    <td align="center"><a href="https://github.com/patricegautier"><img src="https://avatars.githubusercontent.com/u/38435239?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Patrice</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=patricegautier" title="Code">💻</a></td>
    <td align="center"><a href="http://jamesw.link/me"><img src="https://avatars.githubusercontent.com/u/8067792?v=4?s=100" width="100px;" alt=""/><br /><sub><b>James White</b></sub></a><br /><a href="https://github.com/containrrr/watchtower/commits?author=jamesmacwhite" title="Documentation">📖</a></td>
  </tr>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

This project follows the [all-contributors](https://github.com/all-contributors/all-contributors) specification. Contributions of any kind welcome!
