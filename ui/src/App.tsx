import './App.css'
import {BrowserRouter, Route, Routes} from "react-router-dom";
import Availability from "./Availability.tsx";
import SearchMedia from "./SearchMedia.tsx";
import About from "./About.tsx";
import Diff from "./Diff.tsx";
import Intersect from "./Intersect.tsx";
import Unique from "./Unique.tsx";

function App() {
    return (
        <BrowserRouter>
            <Routes>
                <Route path="/" element={<SearchMedia/>}/>
                <Route path="/availability/:mediaId" element={<Availability/>}/>
                <Route path="/diff" element={<Diff/>}/>
                <Route path="/diff/:leftLibraryId/:rightLibraryId" element={<Diff/>}/>
                <Route path="/intersect" element={<Intersect/>}/>
                <Route path="/intersect/:leftLibraryId/:rightLibraryId" element={<Intersect/>}/>
                <Route path="/unique" element={<Unique/>}/>
                <Route path="/unique/:libraryId" element={<Unique/>}/>
                <Route path="/about" element={<About/>}/>
            </Routes>
        </BrowserRouter>
    )
}

export default App
