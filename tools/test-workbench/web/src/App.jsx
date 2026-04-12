import { Routes, Route } from 'react-router-dom'

export default () => {
  return (
    <Routes>
      <Route path="/" element={<Home />} />
    </Routes>
  )
}
