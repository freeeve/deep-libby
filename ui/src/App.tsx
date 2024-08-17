import './App.css'
import {BrowserRouter, Route, Routes} from "react-router-dom";
import Availability from "./Availability.tsx";
import SearchMedia from "./SearchMedia.tsx";

function App() {
    return (
        <BrowserRouter>
            <Routes>
                <Route path="/" element={<SearchMedia/>}/>
                <Route path="/availability/:mediaId" element={<Availability/>}/>
            </Routes>
        </BrowserRouter>
    )
}

export default App
