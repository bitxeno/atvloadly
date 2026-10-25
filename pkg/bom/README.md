# bom

Apple iOS / macOS Assets.car decoder, write in golang.

Forked from [iineva/bom](https://github.com/iineva/bom) at `0e05147`, vendored
into atvloadly at [pkg/bom](.) so the lzfse dependency can be swapped for the
purego [lzfse-go](https://github.com/bitxeno/lzfse-go) decoder, which builds
without cgo.

* BOM: Bill of Materials
* Asset Catalog: Assets.car, and It's a BOM file with special block

### Decode bom file

```golang
import "github.com/bitxeno/atvloadly/pkg/bom/pkg/bom"

fileName := "Assets.car"
f, _ := os.Open(fileName)
defer f.Close()
b := bom.New(f)
err := b.Parse() // parse header first

// read block names
names := b.BlockNames()
// read block
r, err := b.ReadBlock(names[0])
// read tree block
err := b.ReadTree("FACETKEYS", func(k io.Reader, d io.Reader) error {
    // handle tree block item
})
```

### Decode Asset Catalog

```golang
import "github.com/bitxeno/atvloadly/pkg/bom/pkg/asset"

fileName := "Assets.car"
f, _ := os.Open(fileName)
defer f.Close()
b, _ := asset.NewWithReadSeeker(f)
// read image with name
img, err := b.Image("AppIcon")

// read the largest decoded image for the name
largest, err := b.LargestImage("AppIcon")

// read the largest decoded image that matches idiom and scale
phone3x, err := b.ImageWithOptions("AppIcon", asset.ImageOptions{Idiom: "phone", Scale: 3})
```

# Reference

* <https://blog.timac.org/2018/1018-reverse-engineering-the-car-file-format/>
* <https://blog.timac.org/2018/1112-quicklook-plugin-to-visualize-car-files/>
* <https://github.com/hogliux/bomutils>
* <http://lingyuncxb.com/2019/04/14/HumbleAssetCatalog/>
* <https://github.com/lzfse/lzfse>
* <https://github.com/bitxeno/lzfse-go>
