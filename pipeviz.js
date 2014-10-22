var width = window.innerWidth,
    height = window.innerHeight,
    color = d3.scale.category20(),
    nodes = {},
    links = [],
    nlist = [];

var alist = [
    {commit: "virtual-sink", type: "graph-anchor", name: "sink", fixed: true, y: height / 2, x: width - 20},
    {commit: "virtual-source", type: "graph-anchor", name: "source", fixed: true, y: height / 2, x: 20}
];

var g = new graphlib.Graph();

var svg = d3.select('body').append('svg')
    .attr('width', width)
    .attr('height', height);

var force = d3.layout.force()
    .charge(-3000)
    .linkDistance(150)
    .gravity(0)
    .size([width, height]);

// Capture the vertex and edge set as top-level vars
var selectors = {
    "instances": svg.selectAll(".node.instance"),
    "commits": svg.selectAll(".node.commit"),
    "links": svg.selectAll(".link.commit"),
    "anchors": svg.selectAll(".node.anchor"),
    "anchorlinks": svg.selectAll(".link.anchor")
}

d3.json("/fixtures/state2.json", function(err, res) {
    res.cgraph.map(function(e) {
        g.setEdge(e[0], e[1]);
    });

    res.instances.map(function(i) {
        i.type = "instance";
        nodes[i.commit] = i;
        nlist.push(i);
    });

    // find all commits with multiple predecessors or successors; these all must have nodes.
    g.nodes().map(function(commit) {
        // TODO if we see merge commits second+-parents, this logic becomes wrong
        if ((g.predecessors(commit).length > 1 || g.successors(commit).length > 1)
            && !nodes.hasOwnProperty(commit)) {
            nodes[commit] = {
                "name": commit.substring(0, 7),
                "commit": commit,
                "type": "commit"
            };
            nlist.push(nodes[commit]);
        }
    });

    // now traverse depth-first to figure out the overlaid edge structure
    var visited = [], // "black" list - vertices that've been visited
        path = [], // the current path of interstitial commits
        npath = [], // the current path, nodes only
        from, // head of the current exploration path
        v; // vertex (commit) currently being visited


    // the depth-first walker
    var walk = function(v) {
        // guaranteed acyclic, safe to skip grey/back-edge

        var pop_npath = false;
        // grab head of node path from stack
        from = npath[npath.length - 1];

        if (visited.indexOf(v) != -1) {
            // Vertex is black/visited; create link and return. Earlier
            // code SHOULD guarantee this to be a node-point.
            if (v !== from.commit) { // shouldn't happen, but just in case
                links.push({ source: from, target: nodes[v], path: path });
                path = [];
            } else {
                console.log("weird visited walk case");
            }
            return;
        }

        if (from.commit !== v) {
            if (nodes.hasOwnProperty(v)) {
                // Found node point. Create a link
                links.push({ source: from, target: nodes[v], path: path });
                // Our exploration structure inherently guarantees a spanning
                // tree, so we can safely discard historical path information
                path = [];

                // Push newly-found node point onto our npath, it's the new head
                npath.push(nodes[v]);
                // Correspondignly, indicate to pop the npath when exiting
                pop_npath = true;
            }
            else {
                // Not a node point and not self - push commit onto path
                path.push(v);
            }
        }

        // recursive call, the crux of this depth-first traversal
        g.successors(v).map(function(s) {
            walk(s);
        });

        // Mark commit black/visited
        visited.push(v);

        if (pop_npath) {
            npath.pop();
        }
    };

    // walk, working from source commits
    var stack = g.sources()
    while (stack.length !== 0) {
        v = stack.pop();
        npath.push(nodes[v]);
        walk(v);
    }

    g.sinks().map(function(c) {
        links.push({ source: nodes[c], target: alist[0], type: "anchor"});
    })

    g.sources().map(function(c) {
        links.push({ source: alist[1], target: nodes[c], type: "anchor"});
    })

    force.nodes(nlist.concat(alist)).links(links);
    var link = selectors.links.data(links)
        .enter().append("line")
        .attr("class", function(d) {
            return d.type == "anchor" ? "link anchor" : "link commit";
        })
    .style("stroke-width", function(d) {
        return (d.path && d.path.length > 0) ? 4 * Math.sqrt(d.path.length) : 1;
    });

    var anchors = selectors.anchors.data(alist, function(d, i) { return d.commit; })
        .enter().append("g")
        .attr("class", "node anchor");

        anchors.append("circle")
        .attr("x", 0)
        .attr("y", 0);

    var instances = selectors.instances.data(nlist.reduce(function(r, v) {
        if (v.type == "instance") { r.push(v) }
        return r;
    }, []), function(d, i) { return d.commit; })
        .enter().append("g")
        .attr("class", "node instance");

    instances.append("circle")
        .attr("x", 0)
        .attr("y", 0)
        .attr("r", 35)

    instances.append("text")
        .attr("class", "instance-name")
        .text(function(d) { return d.name });

    var commits = selectors.commits.data(nlist.reduce(function(r, v) {
        if (v.type == "commit") { r.push(v) }
        return r;
    }, []), function(d, i) { return d.commit; })
        .enter().append("g")
        .attr("class", "node commit");

    commits.append("circle")
        .attr("x", 0)
        .attr("y", 0)
        .attr("r", 20)

    commits.append("text")
        .attr("class", "instance-name")
        .text(function(d) { return d.commit.substring(0, 7) });

    force.on("tick", function() {
        link.attr("x1", function(d) { return d.source.x; })
        .attr("y1", function(d) { return d.source.y; })
        .attr("x2", function(d) { return d.target.x; })
        .attr("y2", function(d) { return d.target.y; });

        instances.attr("transform", function(d) { return "translate(" + d.x + "," + d.y + ")"; });
        commits.attr("transform", function(d) { return "translate(" + d.x + "," + d.y + ")"; });
        anchors.attr("transform", function(d) { return "translate(" + d.x + "," + d.y + ")"; });
    });

    force.start();
});

