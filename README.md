# LaTTe
Generate PDFs using LaTeX templates and JSON.

LaTTe simply wraps [pdfLaTeX](https://tug.org/applications/pdftex/) and exposes it as a service over HTTP; while also offering some degree of templating/pre-processing, caching and persistence.

## Running LaTTe
LaTTe is available as a docker image. Running 
```
	$ docker run --rm -d -p 27182:27182 raphaelreyna/latte
```
from your terminal will leave you with a running LaTTe instance.

### Image Tags
There are several LaTTe images available to serve a wide range of needs, and they all follow the same tagging convention:
```
	latte:<VERSION>-[pg]-<base/full>
```
where \<VERSION\> denotes the latte version, [pg] if present denotes Postgres support, and \<base/full\> denotes the presence of either [texlive-full](https://packages.ubuntu.com/eoan/texlive-full) or [texlive-base](https://packages.ubuntu.com/eoan/texlive-base).
	
#### Currently Supported Tags
The currently supported tags for LaTTe are:
##### v0.8.1-base
##### v0.8.1-pg-base
##### latest, v0.8.1-full
##### v0.8.1-pg-base

#### Building Custom Images
LaTTe comes with a build script, build/build.sh, which makes it easy to build LaTTe images with custom Go build flags and tex packages.

```
Usage: build.sh [-h] [-s] [-b build_tag] [-p latex_package] [-t image_tag] [-d descriptor] [-H host_name] [-u user_name]

Description: build.sh parametrically builds and tags Docker images for Latte.
             The tag used for the image follows the template bellow:
                 host_name/user_name/image_name:image_tag-descriptor

Flags:
  -b Build tags to be passed to the Go compiler.

  -d Descriptor to be used when tagging the built image.

  -h Show this help text.

  -p LaTeX package to install, must be available in default Ubuntu repos.
     (default: texlive-base)

  -s Skip the image building stage. The generated Dockerfile will be sent to std out.

  -t Tag the Docker image.
     The image will be tagged with the current git tag if this flag is omitted.
     If no git tag is found, we default to using 'latest' as the image tag.

  -u Username to be used when tagging the built image.
     (default: raphaelreyna)

  -H Hostname to be used when tagging the built image.

  -y Do not ask to confirm image tab before building image.
```
### Image size
LaTTe relies on [pdflatex](https://www.tug.org/applications/pdftex/) in order to actually create the PDF files.
Unfortunately, this means that image sizes can be rather large (a full texlive installation is around 4GB).
The build script in the `build` directory makes it easy to create custom sized images of LaTTe to fit your needs.

## How to use LaTTe
LaTTe starts an HTTP server and listens on port 27182 by default at the root endpoint `/`.

### Example
Here we demonstrate how to generate a PDF of the Pythagorean theorem, after substituting variables a, b & c for x, y & z respectively.

We create our .tex template file pythagorean_template.tex:
```
\
The Pythagorean Theorem: $#!.a!# ^ 2 + #!.b!# ^ 2 = #!.c!# ^ 2$
\bye
```
The template `.tex` file should be a template that follows [Go's templating syntax](https://golang.org/pkg/text/template/).
LaTTe currently only accepts using `#!` and `!#` as the left and right delimeters (respectively) in the `.tex` template file. As required by pdfLaTeX, all files must start with the character "\".

We then convert it to base 64:
```
$ cat pythagorean_template.tex | base64
```
which gives the output:
```
XApUaGUgUHl0aGFnb3JlYW4gVGhlb3JlbTogJCMhLmEhIyBeIDIgKyAjIS5iISMgXiAyID0gIyEu
YyEjIF4gMiQKXGJ5ZQo=
```

We then send this to LaTTe:
```
$ curl -X GET -H "Content-Type: application/json" \
-d '{"template":"XApUaGUgUHl0aGFnb3JlYW4gVGhlb3JlbT\
ogJCMhLmEhIyBeIDIgKyAjIS5iISMgXiAyID0gIyEuYyEjIF4gMiQKXGJ5ZQo=", \
"details": { "a": "x", "b": "y", "c": "z" } }' \
--output pythagorean.pdf "http://localhost:27182"
```

which leaves us with the file `pythagorean.pdf`.

## Environment Variables
### `LATTE_PORT`
The port that LaTTe will bind to. The default value is 27182.
### `LATTE_ROOT`
The directory that LaTTe will use to store all of its files. The default value is the users cache directory.
### `LATTE_DB_HOST`
The address where LaTTe can reach its database (assuming LaTTe was compiled with database support).
### `LATTE_DB_PORT`
The the port that LaTTe will use when connecting to its database (assuming LaTTe was compiled with database support).
### `LATTE_DB_USERNAME`
The username that LaTTe will use to connect to its database (assuming LaTTe was compiled with database support).
### `LATTE_DB_PASSWORD`
The password that LaTTe will use to connect to its database (assuming LaTTe was compiled with database support).
### `LATTE_DB_SSL`
Dictates if the database that LaTTe will use is using SSL; acceptable values are `required` and `disable` (assuming LaTTe was compiled with database support).

## Contributing
Contributions are welcome!
### Adding databases / persistent store drivers
LaTTe can easily be extended to support using various databases and other storage solutions.
To have LaTTe use your persistent storage solution of choice, simply create a struct that satisfies the `DB` interface:
```
type DB interface {
	// Store should be capable of storing a given []byte or contents of an io.ReadCloser
	Store(ctx context.Context, uid string, i interface{}) error
	// Fetch should return either a []byte, or io.ReadCloser.
	// If the requested resource could not be found, error should be of type NotFoundError
	Fetch(ctx context.Context, uid string) (interface{}, error)
	// Ping should check if the databases is reachable.
  // If it is, the return error should be nil and non-nil otherwise.
	Ping(ctx context.Context) error
}
```

## Roadmap
- :heavy_check_mark: <s>Registering templates and resources.</s>
- Add support for AWS S3, <s>PostrgeSQL</s>, and possibly other forms of persistent storage.
- CLI tool.
- Add support for building PDFs from multiple LaTeX files.
- Whatever else comes up
