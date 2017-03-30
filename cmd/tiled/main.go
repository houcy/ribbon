package main

import (
	"compress/gzip"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	. "github.com/fogleman/fauxgl"
	"github.com/fogleman/ribbon/pdb"
	"github.com/fogleman/ribbon/ribbon"
	"github.com/nfnt/resize"
)

const (
	size  = 256
	scale = 4
)

func main() {
	args := os.Args[1:]
	if len(args) != 1 || len(args[0]) != 4 {
		fmt.Println("Usage: rcsb XXXX")
		fmt.Println(" XXXX: 4-digit RCSB PDB Structure ID")
		os.Exit(1)
	}
	structureID := args[0]

	var done func()

	done = timed("downloading pdb file")
	models, err := downloadAndParse(structureID)
	if err != nil {
		log.Fatal(err)
	}
	model := models[0]
	done()

	fmt.Printf("atoms       = %d\n", len(model.Atoms))
	fmt.Printf("residues    = %d\n", len(model.Residues))
	fmt.Printf("chains      = %d\n", len(model.Chains))
	fmt.Printf("helixes     = %d\n", len(model.Helixes))
	fmt.Printf("strands     = %d\n", len(model.Strands))
	fmt.Printf("het-atoms   = %d\n", len(model.HetAtoms))
	fmt.Printf("connections = %d\n", len(model.Connections))

	done = timed("generating triangle mesh")
	mesh := ribbon.ModelMesh(model)
	done()

	fmt.Printf("triangles   = %d\n", len(mesh.Triangles))

	done = timed("transforming mesh")
	m := mesh.BiUnitCube()
	done()

	done = timed("finding ideal camera position")
	c := ribbon.PositionCamera(model, m)
	done()

	// done = timed("writing mesh to disk")
	// mesh.SaveSTL(fmt.Sprintf("%s.stl", structureID))
	// done()

	// render
	context := NewContext(1024*scale, 1024*scale)
	for z := 0; z < 8; z++ {
		i := int(math.Pow(2, float64(z)))
		for x := 0; x < i; x++ {
			for y := 0; y < i; y++ {
				done = timed("rendering image")
				context.ClearColorBufferWith(HexColor("1D181F"))
				context.ClearDepthBuffer()
				matrix := LookAt(c.Eye, c.Center, c.Up).Perspective(c.Fovy, c.Aspect, 1, 100)
				matrix = matrix.Viewport(float64(-1-x*2), float64(-1-y*2), float64(2*i), float64(2*i))
				light := c.Eye.Sub(c.Center).Normalize()
				shader := NewPhongShader(matrix, light, c.Eye)
				shader.AmbientColor = Gray(0.3)
				shader.DiffuseColor = Gray(0.9)
				context.Shader = shader
				context.DrawTriangles(mesh.Triangles)
				done()

				// save image
				done = timed("downsampling image")
				image := context.Image()
				image = resize.Resize(1024, 1024, image, resize.Bilinear)
				done()

				done = timed("writing image to disk")
				SavePNG(fmt.Sprintf("tiles/%d.%d.%d.png", z, x, i-y-1), image)
				done()
			}
		}
	}
}

func downloadAndParse(structureID string) ([]*pdb.Model, error) {
	url := fmt.Sprintf(
		"https://files.rcsb.org/download/%s.pdb.gz",
		strings.ToUpper(structureID))
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	r, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}
	return pdb.NewReader(r).ReadAll()
}

func timed(name string) func() {
	if len(name) > 0 {
		fmt.Printf("%s... ", name)
	}
	start := time.Now()
	return func() {
		fmt.Println(time.Since(start))
	}
}